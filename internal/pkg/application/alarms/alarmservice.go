package alarms

import (
	"context"
	"encoding/json"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/models"
	"github.com/diwise/messaging-golang/pkg/messaging"
)

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	Start()
	Stop()

	GetAlarms(ctx context.Context, onlyActive bool) ([]models.Alarm, error)
	AddAlarm(ctx context.Context, alarm models.Alarm) error
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

	as.messenger.RegisterTopicMessageHandler("alarms.batteryLevelWarning", WatchdogBatteryLevelWarningHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("alarms.lastObservedWarning", WatchdogLastObservedWarningHandler(m, as))

	return as
}

func (a *alarmService) Start() {
}

func (a *alarmService) Stop() {
}

func (a *alarmService) GetAlarms(ctx context.Context, onlyActive bool) ([]models.Alarm, error) {
	alarms, err := a.alarmRepository.GetAlarms(ctx, onlyActive)
	if err != nil {
		return nil, err
	}

	return alarms, nil
}
func (a *alarmService) AddAlarm(ctx context.Context, alarm models.Alarm) error {
	return a.alarmRepository.AddAlarm(ctx, alarm)
}

func WatchdogBatteryLevelWarningHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID   string    `json:"deviceID"`
			ObservedAt time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("deviceID", message.DeviceID).Logger()

		err = as.AddAlarm(ctx, models.Alarm{
			RefID: models.AlarmIdentifier{
				DeviceID: message.DeviceID,
			},
			Type:        msg.RoutingKey,
			Severity:    models.AlarmSeverityLow,
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

func WatchdogLastObservedWarningHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID   string    `json:"deviceID"`
			ObservedAt time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("deviceID", message.DeviceID).Logger()

		err = as.AddAlarm(ctx, models.Alarm{
			RefID: models.AlarmIdentifier{
				DeviceID: message.DeviceID,
			},
			Type:        msg.RoutingKey,
			Severity:    models.AlarmSeverityMedium,
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
