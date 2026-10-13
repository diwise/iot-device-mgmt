package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	devicequery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"

	"github.com/diwise/messaging-golang/pkg/messaging"
)

var tracer = otel.Tracer("iot-device-mgmt/alarms")

var ErrAlarmNotFound = fmt.Errorf("alarm not found")

const AlarmDeviceNotObserved string = "device_not_observed"

//go:generate moq -rm -out alarmstorage_mock.go . AlarmStorage
type AlarmStorage interface {
	Add(ctx context.Context, deviceID string, a types.AlarmDetails) error
	Remove(ctx context.Context, deviceID string, alarmType string) error
	Stale(ctx context.Context) (types.Collection[types.Device], error)
	Alarms(ctx context.Context, query alarmquery.Alarms) (types.Collection[types.Alarms], error)
}

type DeviceTenantReader interface {
	Query(ctx context.Context, query devicequery.DeviceFilters) (types.Collection[types.Device], error)
}

type svc struct {
	storage   AlarmStorage
	devices   DeviceTenantReader
	messenger messaging.MsgContext
	config    map[string]types.AlarmType
}

//go:generate moq -rm -out alarmservice_mock.go . AlarmAPIService
type AlarmAPIService interface {
	Add(ctx context.Context, deviceID string, alarm types.AlarmDetails) error
	Remove(ctx context.Context, deviceID string, alarmType string) error
	Stale(ctx context.Context) (types.Collection[types.Device], error)
	Alarms(ctx context.Context, query alarmquery.Alarms) (types.Collection[types.Alarms], error)
}

type Config struct {
	AlarmTypes []types.AlarmType `yaml:"alarmtypes"`
}

func (svc *svc) Add(ctx context.Context, deviceID string, alarm types.AlarmDetails) error {
	log := logging.GetFromContext(ctx)

	alarmType := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(alarm.AlarmType, " ", "_")))

	cfg, ok := svc.config[alarmType]
	if !ok {
		log.Debug("unknown alarm type", "alarm_type", alarmType)
		return nil
	}

	if !cfg.Enabled {
		log.Debug("alarm type is disabled", "alarm_type", alarmType)
		return nil
	}

	alarm.AlarmType = alarmType
	alarm.Severity = cfg.Severity

	if alarm.ObservedAt.IsZero() {
		alarm.ObservedAt = time.Now().UTC()
	}

	return svc.storage.Add(ctx, deviceID, alarm)
}

func (svc *svc) Stale(ctx context.Context) (types.Collection[types.Device], error) {
	return svc.storage.Stale(ctx)
}

func (svc *svc) Remove(ctx context.Context, deviceID string, alarmType string) error {
	return svc.storage.Remove(ctx, deviceID, alarmType)
}

func (svc *svc) Alarms(ctx context.Context, query alarmquery.Alarms) (types.Collection[types.Alarms], error) {
	if !query.ActiveOnly {
		query.ActiveOnly = true
	}

	return svc.storage.Alarms(ctx, query)
}

func New(s AlarmStorage, m messaging.MsgContext, cfg *Config, devices DeviceTenantReader) AlarmAPIService {
	svc := &svc{
		storage:   s,
		devices:   devices,
		messenger: m,
		config:    make(map[string]types.AlarmType),
	}

	for _, at := range cfg.AlarmTypes {
		svc.config[at.Name] = at
	}

	return svc
}

func RegisterTopicMessageHandler(ctx context.Context, svc AlarmAPIService, lookup DeviceTenantReader, messenger messaging.MsgContext) error {
	return messenger.RegisterTopicMessageHandler("device-status", newDeviceStatusHandler(svc, lookup))
}

func newDeviceStatusHandler(svc AlarmAPIService, lookup DeviceTenantReader) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) error {
		var err error

		ctx, span := tracer.Start(ctx, "device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		m := types.StatusMessage{}
		err = json.Unmarshal(itm.Body(), &m)
		if err != nil {
			log.Error("failed to unmarshal status message", "handler", "Alarms.DeviceStatusHandler", "err", err.Error())
			return messaging.Permanent(err)
		}

		if m.Tenant == "" {
			log.Warn("dropping device-status with missing tenant", "handler", "Alarms.DeviceStatusHandler", "device_id", m.DeviceID)
			return nil
		}

		if lookup != nil {
			stored, qerr := lookup.Query(ctx, devicequery.DeviceFilters{DeviceID: m.DeviceID})
			if qerr != nil {
				log.Warn("dropping device-status after lookup failure", "handler", "Alarms.DeviceStatusHandler", "device_id", m.DeviceID, "err", qerr.Error())
				return nil
			}

			if stored.Count != 1 {
				log.Warn("dropping device-status for unknown device", "handler", "Alarms.DeviceStatusHandler", "device_id", m.DeviceID)
				return nil
			}

			if stored.Data[0].Tenant != m.Tenant {
				log.Warn("dropping device-status with tenant mismatch", "handler", "Alarms.DeviceStatusHandler", "device_id", m.DeviceID, "tenant", m.Tenant, "expected_tenant", stored.Data[0].Tenant)
				return nil
			}
		}

		if m.Code == nil && len(m.Messages) == 0 {
			log.Debug("received device status with no error code, will remove any device not observed alarms", "device_id", m.DeviceID)
			err = svc.Remove(ctx, m.DeviceID, AlarmDeviceNotObserved)
			if err != nil {
				log.Debug("could not remove device not observed alarms", "device_id", m.DeviceID, "handler", "Alarms.DeviceStatusHandler", "err", err.Error())
			}
			return nil
		}

		if m.Code != nil && *m.Code != "" {
			err = svc.Add(ctx, m.DeviceID, types.AlarmDetails{
				DeviceID:    m.DeviceID,
				AlarmType:   *m.Code,
				Description: strings.Join(m.Messages, ", "),
				ObservedAt:  m.Timestamp,
			})
		} else if len(m.Messages) > 0 {
			for _, msg := range m.Messages {
				err = svc.Add(ctx, m.DeviceID, types.AlarmDetails{
					DeviceID:   m.DeviceID,
					AlarmType:  msg,
					ObservedAt: m.Timestamp,
				})
			}
		}

		// Bevarad semantik: hanteringsfel loggas och ackas. Klassificering
		// till Temporary/Permanent kräver verifierad idempotens.
		if err != nil {
			log.Error("could not add or update alarm for device", "device_id", m.DeviceID, "handler", "Alarms.DeviceStatusHandler", "err", err.Error())
		}
		return nil
	}
}
