package devicemanagement

import (
	"context"
	"encoding/json"
	"time"

	"github.com/diwise/messaging-golang/pkg/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	r "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
)

func DeviceStatusHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
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

		logger = logger.With().Str("deviceID", status.DeviceID).Logger()

		lastObserved, err := time.Parse(time.RFC3339Nano, status.Timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("device-status contains no valid timestamp")
			lastObserved = time.Now().UTC()
		}

		err = dm.UpdateDeviceStatus(ctx, status.DeviceID, r.DeviceStatus{
			BatteryLevel: status.BatteryLevel,
			LastObserved: lastObserved,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not update status on device")
			return
		}

		err = dm.UpdateDeviceState(ctx, status.DeviceID, r.DeviceState{
			Online:     true,
			State:      r.DeviceStateOK,
			ObservedAt: lastObserved,
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not update state on device")
			return
		}

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}

func AlarmsCreatedHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			Alarm struct {
				ID    uint `json:"id"`
				RefID struct {
					DeviceID string `json:"deviceID,omitempty"`
				} `json:"refID"`
				Severity    int       `json:"severity"`
				ObservedAt  time.Time `json:"observedAt"`
			} `json:"alarm"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		if len(message.Alarm.RefID.DeviceID) == 0 {
			return
		}

		deviceID := message.Alarm.RefID.DeviceID

		logger = logger.With().Str("deviceID", deviceID).Logger()

		d, err := dm.GetDeviceByDeviceID(ctx, deviceID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to retrieve device")
			return
		}

		dm.AddAlarm(ctx, deviceID, r.Alarm{
			AlarmID: int(message.Alarm.ID),
			Severity: message.Alarm.Severity,
			ObservedAt: message.Alarm.ObservedAt,
		})


		dm.UpdateDeviceState(ctx, deviceID, r.DeviceState{
			Online:     d.DeviceState.Online,
			State:      r.DeviceStateUnknown,
			ObservedAt: message.Timestamp,
		})

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}

func AlarmsClosedHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			ID        int       `json:"id"`
			Tenant    string    `json:"tenant"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}		

		err = dm.RemoveAlarm(ctx, message.ID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to remove alarm")
			return
		}	

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}
