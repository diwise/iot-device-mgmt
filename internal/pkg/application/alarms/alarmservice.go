package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/otel"

	"github.com/diwise/messaging-golang/pkg/messaging"
)

var tracer = otel.Tracer("iot-device-mgmt/alarms")

var ErrAlarmNotFound = fmt.Errorf("alarm not found")

const AlarmDeviceNotObserved string = "device_not_observed"

//go:generate moq -rm -out alarmstorage_mock.go . AlarmStorage
type AlarmStorage interface {
	AddAlarm(ctx context.Context, deviceID string, a types.Alarm) error
	RemoveAlarm(ctx context.Context, deviceID string, alarmType string) error
	GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error)
}

func NewAlarmStorage(s storage.Store) AlarmStorage {
	return &alarmStorageImpl{
		s: s,
	}
}

type alarmStorageImpl struct {
	s storage.Store
}

func (s *alarmStorageImpl) AddAlarm(ctx context.Context, deviceID string, a types.Alarm) error {
	return s.s.AddAlarm(ctx, deviceID, a)
}
func (s *alarmStorageImpl) GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error) {
	return s.s.GetStaleDevices(ctx)
}
func (s *alarmStorageImpl) RemoveAlarm(ctx context.Context, deviceID string, alarmType string) error {
	return s.s.RemoveAlarm(ctx, deviceID, alarmType)
}

type alarmSvc struct {
	storage   AlarmStorage
	messenger messaging.MsgContext
}

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	Add(ctx context.Context, deviceID string, alarm types.Alarm) error
	Remove(ctx context.Context, deviceID string, alarmType string) error
	GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error)
	RegisterTopicMessageHandler(ctx context.Context) error
}

func (svc *alarmSvc) Add(ctx context.Context, deviceID string, alarm types.Alarm) error {
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

func New(s AlarmStorage, m messaging.MsgContext) AlarmService {
	svc := &alarmSvc{
		storage:   s,
		messenger: m,
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
			log.Error("failed to unmarshal status message", "err", err.Error())
			return
		}

		if m.Code == nil {
			log.Debug("received device status with no error code, will remove any device not observed alarms", "device_id", m.DeviceID)
			err = svc.Remove(ctx, m.DeviceID, AlarmDeviceNotObserved)
			if err != nil {
				log.Debug("could not remove device not observed alarms", "device_id", m.DeviceID, "err", err.Error())
				return
			}
			return
		}

		log.Debug("received device status", "service", "alarmservice", "body", string(itm.Body()))

		err = svc.Add(ctx, m.DeviceID, types.Alarm{
			AlarmType:   *m.Code,
			Description: strings.Join(m.Messages, ", "),
			ObservedAt:  m.Timestamp,
		})

		if err != nil {
			log.Error("could not add or update alarm for device", "device_id", m.DeviceID, "err", err.Error())
		}
	}
}
