package alarms

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"

	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
)

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	Get(ctx context.Context, offset, limit int, tenants []string) (types.Collection[types.Alarm], error)
	Info(ctx context.Context, offset, limit int, tenants []string) (types.Collection[types.InformationItem], error)
	GetByID(ctx context.Context, alarmID string, tenants []string) (types.Alarm, error)
	GetByRefID(ctx context.Context, refID string, offset, limit int, tenants []string) (types.Collection[types.Alarm], error)
	Add(ctx context.Context, alarm types.Alarm) error
	Close(ctx context.Context, alarmID string, tenants []string) error
}

var ErrAlarmNotFound = fmt.Errorf("alarm not found")

//go:generate moq -rm -out alarmrepository_mock.go . AlarmRepository
type AlarmRepository interface {
	QueryInformation(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.InformationItem], error)
	QueryAlarms(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Alarm], error)
	GetAlarm(ctx context.Context, conditions ...storage.ConditionFunc) (types.Alarm, error)
	AddAlarm(ctx context.Context, alarm types.Alarm) error
	CloseAlarm(ctx context.Context, alarmID, tenant string) error
}

type alarmSvc struct {
	storage   AlarmRepository
	messenger messaging.MsgContext
}

func New(d AlarmRepository, m messaging.MsgContext) AlarmService {
	svc := &alarmSvc{
		storage:   d,
		messenger: m,
	}

	svc.messenger.RegisterTopicMessageHandler("device-status", NewDeviceStatusHandler(m, svc))
	svc.messenger.RegisterTopicMessageHandler("watchdog.deviceNotObserved", NewDeviceNotObservedHandler(m, svc))

	return svc
}

func (svc alarmSvc) Get(ctx context.Context, offset, limit int, tenants []string) (types.Collection[types.Alarm], error) {
	alarms, err := svc.storage.QueryAlarms(ctx, storage.WithOffset(offset), storage.WithLimit(limit), storage.WithTenants(tenants))
	if err != nil {
		return types.Collection[types.Alarm]{}, err
	}

	return alarms, nil
}

func (svc alarmSvc) Info(ctx context.Context, offset, limit int, tenants []string) (types.Collection[types.InformationItem], error) {
	alarms, err := svc.storage.QueryInformation(ctx, storage.WithOffset(offset), storage.WithLimit(limit), storage.WithTenants(tenants))
	if err != nil {
		return types.Collection[types.InformationItem]{}, err
	}

	return alarms, nil
}

func (svc alarmSvc) GetByID(ctx context.Context, alarmID string, tenants []string) (types.Alarm, error) {
	alarm, err := svc.storage.GetAlarm(ctx, storage.WithAlarmID(alarmID), storage.WithTenants(tenants))
	if err != nil {
		return types.Alarm{}, err
	}

	return alarm, nil
}

func (svc alarmSvc) GetByRefID(ctx context.Context, refID string, offset, limit int, tenants []string) (types.Collection[types.Alarm], error) {
	alarms, err := svc.storage.QueryAlarms(ctx, storage.WithRefID(refID), storage.WithTenants(tenants))
	if err != nil {
		return types.Collection[types.Alarm]{}, err
	}

	return alarms, nil
}

func (svc alarmSvc) Add(ctx context.Context, alarm types.Alarm) error {
	if alarm.RefID == "" {
		return fmt.Errorf("no refID is set on alarm")
	}
	if alarm.ID == "" {
		alarm.ID = uuid.NewString()
	}
	if alarm.ObservedAt.IsZero() {
		alarm.ObservedAt = time.Now().UTC()
	}

	err := svc.storage.AddAlarm(ctx, alarm)
	if err != nil {
		return err
	}

	return svc.messenger.PublishOnTopic(ctx, &AlarmCreated{
		Alarm:     alarm,
		Tenant:    alarm.Tenant,
		Timestamp: alarm.ObservedAt,
	})
}

func (svc alarmSvc) Close(ctx context.Context, alarmID string, tenants []string) error {
	alarm, err := svc.storage.GetAlarm(ctx, storage.WithAlarmID(alarmID), storage.WithTenants(tenants), storage.WithDeleted())
	if err != nil {
		if errors.Is(err, storage.ErrDeleted) {
			return nil
		}
		return err
	}

	err = svc.storage.CloseAlarm(ctx, alarmID, alarm.Tenant)
	if err != nil {
		return err
	}

	err = svc.messenger.PublishOnTopic(ctx, &AlarmClosed{
		ID:        alarm.ID,
		Tenant:    alarm.Tenant,
		Timestamp: time.Now().UTC()})

	return err
}
