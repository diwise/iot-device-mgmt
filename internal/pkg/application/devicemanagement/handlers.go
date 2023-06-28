package devicemanagement

import (
	"context"
	"encoding/json"
	"time"

	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	r "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"

	"github.com/samber/lo"
)

func DeviceStatusHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		log := logger.With().Str("handler", "devicemanagement.DeviceStatusHandler").Logger()

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
			log.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		log = log.With().Str("device_id", status.DeviceID).Logger()
		ctx = logging.NewContextWithLogger(ctx, log)

		lastObserved, err := time.Parse(time.RFC3339Nano, status.Timestamp)
		if err != nil {
			log.Error().Err(err).Msg("no valid timestamp")
			return
		}

		_, _, err = lo.AttemptWithDelay(3, 1*time.Second, func(index int, duration time.Duration) error {
			return dm.UpdateDeviceStatus(ctx, status.DeviceID, r.DeviceStatus{
				BatteryLevel: status.BatteryLevel,
				LastObserved: lastObserved,
			})
		})
		if err != nil {
			log.Error().Err(err).Msg("could not update status on device")
			return
		}

		_, _, err = lo.AttemptWithDelay(3, 1*time.Second, func(index int, duration time.Duration) error {
			return dm.UpdateDeviceState(ctx, status.DeviceID, r.DeviceState{
				Online:     true,
				State:      r.DeviceStateOK,
				ObservedAt: lastObserved,
			})
		})
		if err != nil {
			log.Error().Err(err).Msg("could not update state on device")
			return
		}
	}
}

func AlarmsCreatedHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		log := logger.With().Str("handler", "AlarmsCreatedHandler").Logger()

		message := struct {
			Alarm struct {
				ID         uint      `json:"id"`
				RefID      string    `json:"refID"`
				Severity   int       `json:"severity"`
				ObservedAt time.Time `json:"observedAt"`
			} `json:"alarm"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			log.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		if len(message.Alarm.RefID) == 0 {
			return
		}

		deviceID := message.Alarm.RefID
		log = log.With().Str("device_id", deviceID).Int("alarm_id", int(message.Alarm.ID)).Logger()
		ctx = logging.NewContextWithLogger(ctx, log)

		if message.Alarm.ID == 0 {
			log.Error().Msg("alarm ID should not be 0")
			return
		}

		d, err := dm.GetDeviceByDeviceID(ctx, deviceID)
		if err != nil {
			log.Debug().Msg("failed to retrieve device")
			return
		}

		err = dm.AddAlarm(ctx, deviceID, int(message.Alarm.ID), message.Alarm.Severity, message.Alarm.ObservedAt)
		if err != nil {
			log.Debug().Msg("failed to add alarm")
			return
		}

		dm.UpdateDeviceState(ctx, deviceID, r.DeviceState{
			Online:     d.DeviceState.Online,
			State:      r.DeviceStateUnknown,
			ObservedAt: message.Timestamp,
		})
	}
}

func AlarmsClosedHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		log := logger.With().Str("handler", "AlarmsClosedHandler").Logger()

		message := struct {
			ID        int       `json:"id"`
			Tenant    string    `json:"tenant"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			log.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		log = logger.With().Int("alarm_id", message.ID).Logger()
		ctx = logging.NewContextWithLogger(ctx, log)

		err = dm.RemoveAlarm(ctx, message.ID)
		if err != nil {
			log.Error().Err(err).Msg("failed to remove alarm")
			return
		}
	}
}
