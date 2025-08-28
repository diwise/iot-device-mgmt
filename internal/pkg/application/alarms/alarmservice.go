package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
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
	AddAlarm(ctx context.Context, deviceID string, a types.AlarmDetails) error
	RemoveAlarm(ctx context.Context, deviceID string, alarmType string) error
	GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error)
	GetAlarms(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Alarms], error)
}

func NewStorage(s storage.Store) AlarmStorage {
	return &alarmStorageImpl{
		s: s,
	}
}

type alarmStorageImpl struct {
	s storage.Store
}

func (s *alarmStorageImpl) AddAlarm(ctx context.Context, deviceID string, a types.AlarmDetails) error {
	return s.s.AddAlarm(ctx, deviceID, a)
}
func (s *alarmStorageImpl) GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error) {
	return s.s.GetStaleDevices(ctx)
}
func (s *alarmStorageImpl) RemoveAlarm(ctx context.Context, deviceID string, alarmType string) error {
	return s.s.RemoveAlarm(ctx, deviceID, alarmType)
}
func (s *alarmStorageImpl) GetAlarms(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Alarms], error) {
	return s.s.GetAlarms(ctx, conditions...)
}

type alarmSvc struct {
	storage   AlarmStorage
	messenger messaging.MsgContext
	config    map[string]types.AlarmType
}

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	Add(ctx context.Context, deviceID string, alarm types.AlarmDetails) error
	Remove(ctx context.Context, deviceID string, alarmType string) error
	GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error)
	RegisterTopicMessageHandler(ctx context.Context) error
	GetAlarms(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Alarms], error)
}

type AlarmServiceConfig struct {
	AlarmTypes []types.AlarmType `yaml:"alarmtypes"`
}

func (svc *alarmSvc) Add(ctx context.Context, deviceID string, alarm types.AlarmDetails) error {
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

	return svc.storage.AddAlarm(ctx, deviceID, alarm)
}

func (svc *alarmSvc) GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error) {
	return svc.storage.GetStaleDevices(ctx)
}

func (svc *alarmSvc) Remove(ctx context.Context, deviceID string, alarmType string) error {
	return svc.storage.RemoveAlarm(ctx, deviceID, alarmType)
}

func (svc *alarmSvc) RegisterTopicMessageHandler(ctx context.Context) error {
	return svc.messenger.RegisterTopicMessageHandler("device-status", NewDeviceStatusHandler(svc))
}

func (svc *alarmSvc) GetAlarms(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Alarms], error) {
	conditions := []storage.ConditionFunc{}

	for k, v := range params {
		switch strings.ToLower(k) {
		case "limit":
			if i, err := strconv.Atoi(v[0]); err == nil {
				conditions = append(conditions, storage.WithLimit(i))
			}
		case "offset":
			if o, err := strconv.Atoi(v[0]); err == nil {
				conditions = append(conditions, storage.WithOffset(o))
			}
		case "alarmtype":
			if v[0] != "" {
				conditions = append(conditions, storage.WithAlarmType(v[0]))
			}
		}
	}

	conditions = append(conditions, storage.WithActive(true), storage.WithTenants(tenants))

	return svc.storage.GetAlarms(ctx, conditions...)
}

func New(s AlarmStorage, m messaging.MsgContext, cfg *AlarmServiceConfig) AlarmService {
	svc := &alarmSvc{
		storage:   s,
		messenger: m,
		config:    make(map[string]types.AlarmType),
	}

	for _, at := range cfg.AlarmTypes {
		svc.config[at.Name] = at
	}

	return svc
}

func NewDeviceStatusHandler(svc AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		m := types.StatusMessage{}
		err = json.Unmarshal(itm.Body(), &m)
		if err != nil {
			log.Error("failed to unmarshal status message", "handler", "Alarms.DeviceStatusHandler", "err", err.Error())
			return
		}

		if m.Code == nil && len(m.Messages) == 0 {
			log.Debug("received device status with no error code, will remove any device not observed alarms", "device_id", m.DeviceID)
			err = svc.Remove(ctx, m.DeviceID, AlarmDeviceNotObserved)
			if err != nil {
				log.Debug("could not remove device not observed alarms", "device_id", m.DeviceID, "handler", "Alarms.DeviceStatusHandler", "err", err.Error())
				return
			}
			return
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

		if err != nil {
			log.Error("could not add or update alarm for device", "device_id", m.DeviceID, "handler", "Alarms.DeviceStatusHandler", "err", err.Error())
		}
	}
}
