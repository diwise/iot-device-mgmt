package devicemanagement

import (
	"context"
	"encoding/json"
	"time"

	"log/slog"

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

		status := struct {
			DeviceID  string `json:"deviceID"`
			Tenant    string `json:"tenant,omitempty"`
			Timestamp string `json:"timestamp"`
		}{}

		err = json.Unmarshal(itm.Body(), &status)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		observedAt, err := time.Parse(time.RFC3339Nano, status.Timestamp)
		if err != nil {
			log.Error("no valid timestamp", "err", err.Error())
			return
		}

		device, err := svc.GetDeviceByDeviceID(ctx, status.DeviceID, []string{status.Tenant})
		if err != nil {
			log.Error("could not fetch device", "err", err.Error())
			return
		}

		ds := device.DeviceStatus
		if observedAt.After(ds.ObservedAt) {
			ds.ObservedAt = observedAt
			err := svc.UpdateDeviceStatus(ctx, device.DeviceID, device.Tenant, ds)
			log.Error("could not update status", "err", err.Error())
			return
		}
	}
}
