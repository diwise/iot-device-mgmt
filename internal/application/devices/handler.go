package devices

import (
	"context"
	"encoding/json"
	"log/slog"

	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/device")

func RegisterTopicMessageHandler(ctx context.Context, svc DeviceStatusHandler, messenger messaging.MsgContext) error {
	return messenger.RegisterTopicMessageHandler("device-status", newDeviceStatusHandler(svc))
}

func (s service) Handle(ctx context.Context, status types.StatusMessage) error {
	log := logging.GetFromContext(ctx)

	if status.Tenant == "" {
		log.Warn("dropping device-status with missing tenant", "device_id", status.DeviceID)
		return nil
	}

	stored, err := s.reader.Query(ctx, dmquery.DeviceFilters{DeviceID: status.DeviceID})
	if err != nil {
		log.Warn("dropping device-status after lookup failure", "device_id", status.DeviceID, "err", err.Error())
		return nil
	}

	if stored.Count != 1 {
		log.Warn("dropping device-status for unknown device", "device_id", status.DeviceID)
		return nil
	}

	if stored.Data[0].Tenant != status.Tenant {
		log.Warn("dropping device-status with tenant mismatch",
			"device_id", status.DeviceID,
			"tenant", status.Tenant,
			"expected_tenant", stored.Data[0].Tenant)
		return nil
	}

	state := types.DeviceState{
		Online:     true,
		State:      types.DeviceStateOK,
		ObservedAt: status.Timestamp,
	}

	if status.Code != nil {
		state.State = types.DeviceStateWarning
	}

	err = s.statusWriter.SetDeviceState(ctx, status.DeviceID, state)
	if err != nil {
		return err
	}

	if status.BatteryLevel == nil && status.DR == nil && status.Frequency == nil && status.LoRaSNR == nil && status.RSSI == nil && status.SpreadingFactor == nil {
		return nil
	}

	return s.statusWriter.AddDeviceStatus(ctx, status)
}

func newDeviceStatusHandler(svc DeviceStatusHandler) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) error {
		var err error

		ctx, span := tracer.Start(ctx, "device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		m := types.StatusMessage{}
		err = json.Unmarshal(itm.Body(), &m)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return messaging.Permanent(err)
		}

		ctx = logging.NewContextWithLogger(ctx, log, slog.String("device_id", m.DeviceID), slog.String("tenant", m.Tenant))

		// Bevarad semantik: hanteringsfel loggas och ackas. Klassificering
		// till Temporary/Permanent kräver verifierad idempotens.
		err = svc.Handle(ctx, m)
		if err != nil {
			log.Error("could not add device status", "err", err.Error())
		}
		return nil
	}
}
