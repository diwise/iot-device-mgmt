package devicemanagement

import (
	"context"
	"encoding/json"
	"slices"
	"time"

	"log/slog"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	models "github.com/diwise/iot-device-mgmt/pkg/types"

	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/device")

func NewDeviceStatusHandler(messenger messaging.MsgContext, svc DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		deviceStatus := struct {
			DeviceID  string `json:"deviceID"`
			Tenant    string `json:"tenant,omitempty"`
			Timestamp string `json:"timestamp"`
		}{}

		err = json.Unmarshal(itm.Body(), &deviceStatus)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		observedAt, err := time.Parse(time.RFC3339Nano, deviceStatus.Timestamp)
		if err != nil {
			log.Error("no valid timestamp", "err", err.Error())
			return
		}

		device, err := svc.GetByDeviceID(ctx, deviceStatus.DeviceID, []string{deviceStatus.Tenant})
		if err != nil {
			log.Error("could not fetch device", "err", err.Error())
			return
		}

		status := device.DeviceStatus
		if observedAt.After(status.ObservedAt) {
			status.ObservedAt = observedAt
			err := svc.UpdateStatus(ctx, device.DeviceID, device.Tenant, status)
			log.Error("could not update status", "err", err.Error())
			return
		}

		state := device.DeviceState
		if observedAt.After(state.ObservedAt) {
			state.ObservedAt = observedAt
			state.Online = true
			state.State = models.DeviceStateOK
		}
	}
}

func NewAlarmCreatedHandler(svc DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "alarm-created")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		a := struct {
			Alarm     types.Alarm `json:"alarm"`
			Tenant    string      `json:"tenant"`
			Timestamp time.Time   `json:"timestamp"`
		}{}

		err = json.Unmarshal(itm.Body(), &a)
		if err != nil {
			log.Error("failed to unmarshal alarm", "err", err.Error())
			return
		}

		device, err := svc.GetByDeviceID(ctx, a.Alarm.RefID, []string{a.Tenant})
		if err != nil {
			log.Error("could not get device by alarm refID", "device_id", a.Alarm.RefID, "err", err.Error())
			return
		}

		if slices.Contains(device.Alarms, a.Alarm.ID) {
			log.Debug("alarm exists")
			return
		}

		device.Alarms = append(device.Alarms, a.Alarm.ID)
		if a.Timestamp.After(device.DeviceState.ObservedAt) {
			device.DeviceState.State = types.DeviceStateWarning
			device.DeviceState.ObservedAt = a.Timestamp
		}

		err = svc.Update(ctx, device)
		if err != nil {
			log.Error("could not update device")
			return
		}
	}
}

func NewAlarmClosedHandler(svc DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "alarm-closed")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		a := struct {
			ID        string    `json:"id"`
			Tenant    string    `json:"tenant"`
			Timestamp time.Time `json:"timestamp"`
		}{}

		err = json.Unmarshal(itm.Body(), &a)
		if err != nil {
			log.Error("failed to unmarshal alarm", "err", err.Error())
			return
		}

		device, err := svc.GetWithAlarmID(ctx, a.ID, []string{a.Tenant})
		if err != nil {
			log.Error("could not get device by alarm id", "alarm_id", a.ID, "err", err.Error())
			return
		}

		device.Alarms = slices.DeleteFunc(device.Alarms, func(s string) bool {
			return s == a.ID
		})

		if len(device.Alarms) == 0 {
			device.DeviceState.State = models.DeviceStateOK
			device.DeviceState.ObservedAt = a.Timestamp
		}

		err = svc.Update(ctx, device)
		if err != nil {
			log.Error("could not update device")
			return
		}
	}
}
