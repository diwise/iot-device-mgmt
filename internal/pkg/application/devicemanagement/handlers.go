package devicemanagement

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"log/slog"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/senml"

	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/device")

func NewDeviceStatusHandler(svc DeviceManagement) messaging.TopicMessageHandler {
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

		ctx = logging.NewContextWithLogger(ctx, log, slog.String("device_id", deviceStatus.DeviceID), slog.String("tenant", deviceStatus.Tenant))

		observedAt, err := time.Parse(time.RFC3339, deviceStatus.Timestamp)
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
		if observedAt.After(status.ObservedAt) || status.ObservedAt.IsZero() {
			status.ObservedAt = observedAt
			err := svc.UpdateStatus(ctx, device.DeviceID, device.Tenant, status)
			if err != nil {
				log.Error("could not update status", "err", err.Error())
				return
			}
		}

		state := device.DeviceState
		if observedAt.After(state.ObservedAt) || state.ObservedAt.IsZero() {
			state.ObservedAt = observedAt
			state.Online = true
			state.State = types.DeviceStateOK
			err := svc.UpdateState(ctx, device.DeviceID, device.Tenant, state)
			if err != nil {
				log.Error("could not update state", "err", err.Error())
				return
			}
		}
	}
}

func NewMessageAcceptedHandler(svc DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "message-accepted")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		message := struct {
			Pack senml.Pack `json:"pack"`
		}{}

		err = json.Unmarshal(itm.Body(), &message)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		getObjectURN := func(m senml.Pack) string {
			r, ok := m.GetStringValue(senml.FindByName("0"))
			if !ok {
				return ""
			}
			return r
		}

		if getObjectURN(message.Pack) != "urn:oma:lwm2m:ext:3" { // only accept Device Object
			return
		}

		batteryLevel, ok := message.Pack.GetValue(senml.FindByName("9"))
		if !ok {
			log.Debug("no battery level found")
			return
		}

		getDeviceID := func(m senml.Pack) string {
			r, ok := m.GetRecord(senml.FindByName("0"))
			if !ok {
				return ""
			}
			return strings.Split(r.Name, "/")[0]
		}

		getTenant := func(m senml.Pack) string {
			r, ok := m.GetRecord(senml.FindByName("tenant"))
			if !ok {
				return ""
			}
			return r.StringValue
		}

		var ts time.Time
		ts, ok = message.Pack.GetTime(senml.FindByName("9"))
		if !ok {
			ts = time.Now().UTC()
		}

		status := types.DeviceStatus{
			BatteryLevel: int(batteryLevel),
			ObservedAt:   ts.UTC(),
		}

		tenant := getTenant(message.Pack)
		deviceID := getDeviceID(message.Pack)

		log.Debug("received battery level", "device_id", deviceID, "battery_level", status.BatteryLevel, "tenant", tenant)

		err = svc.UpdateStatus(ctx, deviceID, tenant, status)
		if err != nil {
			log.Error("could not update status/batterylevel", "err", err.Error())
			return
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
			if errors.Is(err, ErrDeviceNotFound) {
				return
			}
			log.Error("could not get device by alarm id", "alarm_id", a.ID, "err", err.Error())
			return
		}

		device.Alarms = slices.DeleteFunc(device.Alarms, func(s string) bool {
			return s == a.ID
		})

		if len(device.Alarms) == 0 {
			device.DeviceState.State = types.DeviceStateOK
			device.DeviceState.ObservedAt = a.Timestamp
		}

		err = svc.Update(ctx, device)
		if err != nil {
			log.Error("could not update device", "err", err.Error())
			return
		}
	}
}
