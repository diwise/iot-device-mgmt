package alarms

import (
	"context"
	"fmt"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	alarmStorage "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/alarms"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
)

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	GetAlarms(ctx context.Context, offset, limit int, tenants []string) (repositories.Collection[models.Alarm], error)
	GetAlarmByID(ctx context.Context, alarmID string, tenants []string) (models.Alarm, error)
	GetAlarmsByRefID(ctx context.Context, refID string, offset, limit int, tenants []string) (repositories.Collection[models.Alarm], error)
	Add(ctx context.Context, alarm models.Alarm) error
	Close(ctx context.Context, alarmID string, tenants []string) error
}

type alarmSvc struct {
	storage   alarmStorage.AlarmRepository
	messenger messaging.MsgContext
}

func New(d alarmStorage.AlarmRepository, m messaging.MsgContext) AlarmService {
	svc := &alarmSvc{
		storage:   d,
		messenger: m,
	}

	svc.messenger.RegisterTopicMessageHandler("device-status", NewDeviceStatusHandler(m, svc))
	svc.messenger.RegisterTopicMessageHandler("watchdog.deviceNotObserved", NewDeviceNotObservedHandler(m, svc))

	return svc
}

func (svc alarmSvc) GetAlarms(ctx context.Context, offset, limit int, tenants []string) (repositories.Collection[models.Alarm], error) {
	alarms, err := svc.storage.GetAll(ctx, offset, limit, tenants)
	if err != nil {
		return repositories.Collection[models.Alarm]{}, err
	}

	return alarms, nil
}

func (svc alarmSvc) GetAlarmByID(ctx context.Context, alarmID string, tenants []string) (models.Alarm, error) {
	alarm, err := svc.storage.GetByID(ctx, alarmID, tenants)
	if err != nil {
		return models.Alarm{}, err
	}

	return alarm, nil
}

func (svc alarmSvc) GetAlarmsByRefID(ctx context.Context, refID string, offset, limit int, tenants []string) (repositories.Collection[models.Alarm], error) {
	alarms, err := svc.storage.GetByRefID(ctx, refID, offset, limit, tenants)
	if err != nil {
		return repositories.Collection[models.Alarm]{}, err
	}
	return alarms, nil
}

func (svc alarmSvc) Add(ctx context.Context, alarm models.Alarm) error {
	if alarm.ID == "" {
		alarm.ID = uuid.NewString()
	}
	if alarm.RefID == "" {
		return fmt.Errorf("no refID is set on alarm")
	}
	if alarm.ObservedAt.IsZero() {
		alarm.ObservedAt = time.Now().UTC()
	}

	alarm.Type = "Alarm"

	err := svc.storage.Add(ctx, alarm, alarm.Tenant)
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
	alarm, err := svc.storage.GetByID(ctx, alarmID, tenants)
	if err != nil {
		return err
	}

	err = svc.storage.Close(ctx, alarmID, alarm.Tenant)
	if err != nil {
		return err
	}

	err = svc.messenger.PublishOnTopic(ctx, &AlarmClosed{
		ID:        alarm.ID,
		Tenant:    alarm.Tenant,
		Timestamp: time.Now().UTC()})

	return err
}
