package alarms

import (
	"context"
	"encoding/json"
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
}

type alarmService struct {
	alarmRepository db.AlarmRepository
	messenger       messaging.MsgContext
}

func New(d db.AlarmRepository, m messaging.MsgContext) AlarmService {
	as := &alarmService{
		alarmRepository: d,
		messenger:       m,
	}

	as.messenger.RegisterTopicMessageHandler("watchdog.batteryLevelChanged", BatteryLevelChangedHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("watchdog.deviceNotObserved", DeviceNotObservedHandler(m, as))

	return as
}

func (a *alarmService) Start() {
}

func (a *alarmService) Stop() {
}

func (a *alarmService) GetAlarms(ctx context.Context, onlyActive bool) ([]db.Alarm, error) {
	alarms, err := a.alarmRepository.GetAll(ctx, onlyActive)
	if err != nil {
		return nil, err
	}

	return alarms, nil
}
func (a *alarmService) AddAlarm(ctx context.Context, alarm db.Alarm) error {
	return a.alarmRepository.Add(ctx, alarm)
}
func (a *alarmService) CloseAlarm(ctx context.Context, alarmID int) error {
	err := a.alarmRepository.Close(ctx, alarmID)
	if err != nil {
		return err
	}
	return a.messenger.PublishOnTopic(ctx, &AlarmClosed{ID: alarmID, Timestamp: time.Now().UTC()})
}

func BatteryLevelChangedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
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
			Type:        msg.RoutingKey,
			Severity:    db.AlarmSeverityLow,
			Active:      true,
			Description: "",
			ObservedAt:  message.ObservedAt,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not add alarm")
			return
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
			Type:        msg.RoutingKey,
			Severity:    db.AlarmSeverityMedium,
			Active:      true,
			Description: "",
			ObservedAt:  message.ObservedAt,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not add alarm")
			return
		}

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}
