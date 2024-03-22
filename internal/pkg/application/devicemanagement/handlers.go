package devicemanagement

import (
	"context"
	"encoding/json"
	"time"

	"log/slog"

	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	r "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"

	"github.com/samber/lo"
)

func DeviceStatusHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		log := l.With(slog.String("handler", "devicemanagement.DeviceStatusHandler"))

		status := struct {
			DeviceID     string   `json:"deviceID"`
			BatteryLevel int      `json:"batteryLevel"`
			Code         int      `json:"statusCode"`
			Messages     []string `json:"statusMessages,omitempty"`
			Tenant       string   `json:"tenant,omitempty"`
			Timestamp    string   `json:"timestamp"`
		}{}

		err := json.Unmarshal(itm.Body(), &status)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		log = log.With(slog.String("device_id", status.DeviceID))
		ctx = logging.NewContextWithLogger(ctx, log)

		lastObserved, err := time.Parse(time.RFC3339Nano, status.Timestamp)
		if err != nil {
			log.Error("no valid timestamp", "err", err.Error())
			return
		}

		_, _, err = lo.AttemptWithDelay(3, 1*time.Second, func(index int, duration time.Duration) error {
			return dm.UpdateDeviceStatus(ctx, status.DeviceID, r.DeviceStatus{
				BatteryLevel: status.BatteryLevel,
				LastObserved: lastObserved,
			})
		})
		if err != nil {
			log.Error("could not update status on device", "err", err.Error())
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
			log.Error("could not update state on device", "err", err.Error())
			return
		}
	}
}

func AlarmsCreatedHandler(messenger messaging.MsgContext, dm DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		log := l.With(slog.String("handler", "AlarmsCreatedHandler"))

		message := struct {
			Alarm struct {
				ID         uint      `json:"id"`
				RefID      string    `json:"refID"`
				Severity   int       `json:"severity"`
				ObservedAt time.Time `json:"observedAt"`
			} `json:"alarm"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err := json.Unmarshal(itm.Body(), &message)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		if len(message.Alarm.RefID) == 0 {
			return
		}

		deviceID := message.Alarm.RefID
		log = log.With(
			slog.String("device_id", deviceID),
			slog.Int("alarm_id", int(message.Alarm.ID)),
		)
		ctx = logging.NewContextWithLogger(ctx, log)

		if message.Alarm.ID == 0 {
			log.Error("alarm ID should not be 0")
			return
		}

		d, err := dm.GetDeviceByDeviceID(ctx, deviceID)
		if err != nil {
			log.Debug("failed to retrieve device")
			return
		}

		err = dm.AddAlarm(ctx, deviceID, int(message.Alarm.ID), message.Alarm.Severity, message.Alarm.ObservedAt)
		if err != nil {
			log.Debug("failed to add alarm")
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
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		log := l.With(slog.String("handler", "AlarmsClosedHandler"))

		message := struct {
			ID        int       `json:"id"`
			Tenant    string    `json:"tenant"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err := json.Unmarshal(itm.Body(), &message)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		log = log.With(slog.Int("alarm_id", message.ID))
		ctx = logging.NewContextWithLogger(ctx, log)

		err = dm.RemoveAlarm(ctx, message.ID)
		if err != nil {
			log.Error("failed to remove alarm", "err", err.Error())
			return
		}
	}
}
