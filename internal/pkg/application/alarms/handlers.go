package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	"github.com/diwise/messaging-golang/pkg/messaging"
)

func BatteryLevelChangedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger = logger.With().Str("handler", "BatteryLevelChangedHandler").Logger()
		
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

		logger = logger.With().Str("device_id", message.DeviceID).Logger()

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

		logger.Debug().Msg("Ok")
	}
}

func DeviceNotObservedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger = logger.With().Str("handler", "DeviceNotObservedHandler").Logger()

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

		logger = logger.With().Str("device_id", message.DeviceID).Logger()

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

		logger.Debug().Msg("Ok")
	}
}

func DeviceStatusHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger = logger.With().Str("handler", "DeviceStatusHandler").Logger()

		message := struct {
			DeviceID     string   `json:"deviceID"`
			BatteryLevel int      `json:"batteryLevel"`
			Code         int      `json:"statusCode"`
			Messages     []string `json:"statusMessages,omitempty"`
			Tenant       string   `json:"tenant"`
			Timestamp    string   `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		if message.DeviceID == "" {
			logger.Error().Msg("no device information")
			return
		}

		logger = logger.With().Str("device_id", message.DeviceID).Logger()

		if message.Code == 0 {
			return
		}

		ts, err := time.Parse(time.RFC3339Nano, message.Timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("no valid timestamp")
			return
		}

		if message.Tenant == "" {
			logger.Error().Msg("no tenant information")
			return
		}

		if len(message.Messages) > 0 {
			alarmType := message.Messages[0]
			desc := strings.Join(message.Messages, "\n ")

			err = as.AddAlarm(ctx, db.Alarm{
				RefID: db.AlarmIdentifier{
					DeviceID: message.DeviceID,
				},
				Type:        alarmType,
				Severity:    db.AlarmSeverityMedium,
				Active:      true,
				Tenant:      message.Tenant,
				ObservedAt:  ts,
				Description: desc,
			})
			if err != nil {
				logger.Error().Err(err).Msg("could not add alarm")
				return
			}
		} else {
			err = as.AddAlarm(ctx, db.Alarm{
				RefID: db.AlarmIdentifier{
					DeviceID: message.DeviceID,
				},
				Type:        fmt.Sprintf("code: %d", message.Code),
				Severity:    db.AlarmSeverityMedium,
				Active:      true,
				Tenant:      message.Tenant,
				ObservedAt:  ts,
				Description: fmt.Sprintf("code: %d", message.Code),
			})
			if err != nil {
				logger.Error().Err(err).Msg("could not add alarm")
				return
			}
		}

		logger.Debug().Msg("Ok")
	}
}
