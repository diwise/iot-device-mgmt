package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms/events"
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	"github.com/diwise/messaging-golang/pkg/messaging"
)

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	Start()
	Stop()

	GetAlarms(ctx context.Context, onlyActive bool) ([]db.Alarm, error)
	AddAlarm(ctx context.Context, alarm db.Alarm) error
	CloseAlarm(ctx context.Context, alarmID int) error

	GetConfiguration() Configuration
}

type alarmService struct {
	alarmRepository db.AlarmRepository
	messenger       messaging.MsgContext
	config          *Configuration
}

func New(d db.AlarmRepository, m messaging.MsgContext, cfg *Configuration) AlarmService {
	as := &alarmService{
		alarmRepository: d,
		messenger:       m,
		config:          cfg,
	}

	as.messenger.RegisterTopicMessageHandler("device-status", DeviceStatusHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("watchdog.batteryLevelChanged", BatteryLevelChangedHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("watchdog.deviceNotObserved", DeviceNotObservedHandler(m, as))

	return as
}

func (a *alarmService) Start() {}
func (a *alarmService) Stop()  {}

func (a *alarmService) GetAlarms(ctx context.Context, onlyActive bool) ([]db.Alarm, error) {
	alarms, err := a.alarmRepository.GetAll(ctx, onlyActive)
	if err != nil {
		return nil, err
	}

	return alarms, nil
}

func (a *alarmService) AddAlarm(ctx context.Context, alarm db.Alarm) error {
	err := a.alarmRepository.Add(ctx, alarm)
	if err != nil {
		return err
	}

	return a.messenger.PublishOnTopic(ctx, &AlarmCreated{
		Alarm:     alarm,
		Tenant:    alarm.Tenant,
		Timestamp: alarm.ObservedAt,
	})
}

func (a *alarmService) CloseAlarm(ctx context.Context, alarmID int) error {
	alarm, err := a.alarmRepository.GetByID(ctx, alarmID)
	if err != nil {
		return err
	}

	if !alarm.Active {
		return nil
	}

	err = a.alarmRepository.Close(ctx, alarmID)
	if err != nil {
		return err
	}

	return a.messenger.PublishOnTopic(ctx, &AlarmClosed{ID: alarmID, Tenant: alarm.Tenant, Timestamp: time.Now().UTC()})
}

func (a *alarmService) GetConfiguration() Configuration {
	return *a.config
}

func BatteryLevelChangedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID     string    `json:"deviceID"`
			BatteryLevel int       `json:"batteryLevel"`
			Tenant       string    `json:"tenant"`
			ObservedAt   time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("deviceID", message.DeviceID).Logger()

		for _, cfg := range as.GetConfiguration().DeviceAlarmConfigurations {
			if strings.EqualFold(cfg.DeviceID, message.DeviceID) {
				if cfg.Name == "batteryLevel" && message.BatteryLevel < int(cfg.Min) {
					err := as.AddAlarm(ctx, db.Alarm{
						RefID: db.AlarmIdentifier{
							DeviceID: message.DeviceID,
						},
						Type:        cfg.Type,
						Severity:    cfg.Severity,
						Active:      true,
						Tenant:      message.Tenant,
						ObservedAt:  time.Now().UTC(),
						Description: fmt.Sprintf("Batterinivå låg %d (min: %d)", message.BatteryLevel, int(cfg.Min)),
					})
					if err != nil {
						logger.Error().Err(err).Msg("could not add alarm")
						return
					}
					break
				}
			}
		}

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}

func DeviceNotObservedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID   string    `json:"deviceID"`
			Tenant     string    `json:"tenant"`
			ObservedAt time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("deviceID", message.DeviceID).Logger()

		err = as.AddAlarm(ctx, db.Alarm{
			RefID: db.AlarmIdentifier{
				DeviceID: message.DeviceID,
			},
			Type:        "deviceNotObserved",
			Severity:    db.AlarmSeverityMedium,
			Active:      true,
			Tenant:      message.Tenant,
			ObservedAt:  time.Now().UTC(),
			Description: fmt.Sprintf("Ingen kommunikation registrerad från %s", message.DeviceID),
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not add alarm")
			return
		}

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}

func DeviceStatusHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		status := struct {
			DeviceID     string   `json:"deviceID"`
			BatteryLevel int      `json:"batteryLevel"`
			Code         int      `json:"statusCode"`
			Messages     []string `json:"statusMessages,omitempty"`
			Tenant       string   `json:"tenant,omitempty"`
			Timestamp    string   `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &status)
		if err != nil {
			logger.Error().Err(err).Msgf("failed to unmarshal message from %s", msg.RoutingKey)
			return
		}

		if status.Code == 0 {
			return
		}

		logger = logger.With().Str("deviceID", status.DeviceID).Logger()

		ts, err := time.Parse(time.RFC3339Nano, status.Timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("device-status contains no valid timestamp")
			ts = time.Now().UTC()
		}

		if len(status.Messages) > 0 {
			for _, m := range status.Messages {
				err = as.AddAlarm(ctx, db.Alarm{
					RefID: db.AlarmIdentifier{
						DeviceID: status.DeviceID,
					},
					Type:        m,
					Severity:    db.AlarmSeverityMedium,
					Active:      true,
					Tenant:      status.Tenant,
					ObservedAt:  ts,
					Description: m,
				})
				if err != nil {
					logger.Error().Err(err).Msg("could not add alarm")
					return
				}
			}
		} else {
			err = as.AddAlarm(ctx, db.Alarm{
				RefID: db.AlarmIdentifier{
					DeviceID: status.DeviceID,
				},
				Type:        fmt.Sprintf("code: %d", status.Code),
				Severity:    db.AlarmSeverityMedium,
				Active:      true,
				Tenant:      status.Tenant,
				ObservedAt:  ts,
				Description: fmt.Sprintf("code: %d", status.Code),
			})
			if err != nil {
				logger.Error().Err(err).Msg("could not add alarm")
				return
			}
		}
	}
}
