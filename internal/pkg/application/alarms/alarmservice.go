package alarms

import (
	"context"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms/events"
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	Start()
	Stop()

	GetAlarms(ctx context.Context) ([]db.Alarm, error)
	AddAlarm(ctx context.Context, alarm db.Alarm) error
	CloseAlarm(ctx context.Context, alarmID int) error

	GetConfiguration() Configuration
}

type alarmService struct {
	alarmRepository db.AlarmRepository
	messenger       messaging.MsgContext
	config          *Configuration
}

func New(d db.AlarmRepository, m messaging.MsgContext, cfg *Configuration) AlarmService {
	as := &alarmService{
		alarmRepository: d,
		messenger:       m,
		config:          cfg,
	}

	as.messenger.RegisterTopicMessageHandler("device-status", DeviceStatusHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("watchdog.batteryLevelChanged", BatteryLevelChangedHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("watchdog.deviceNotObserved", DeviceNotObservedHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("function.updated", FunctionUpdatedHandler(m, as))

	return as
}

func (a *alarmService) Start() {}
func (a *alarmService) Stop()  {}

func (a *alarmService) GetAlarms(ctx context.Context) ([]db.Alarm, error) {
	alarms, err := a.alarmRepository.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	return alarms, nil
}

func (a *alarmService) AddAlarm(ctx context.Context, alarm db.Alarm) error {
	err := a.alarmRepository.Add(ctx, alarm)
	if err != nil {
		return err
	}

	return a.messenger.PublishOnTopic(ctx, &AlarmCreated{
		Alarm:     alarm,
		Tenant:    alarm.Tenant,
		Timestamp: alarm.ObservedAt,
	})
}

func (a *alarmService) CloseAlarm(ctx context.Context, alarmID int) error {
	logger := logging.GetFromContext(ctx)

	alarm, err := a.alarmRepository.GetByID(ctx, alarmID)
	if err != nil {
		logger.Debug().Msgf("alarm %d could not be fetched by ID", alarmID)
		return err
	}

	err = a.alarmRepository.Close(ctx, alarmID)
	if err != nil {
		return err
	}

	return a.messenger.PublishOnTopic(ctx, &AlarmClosed{ID: alarmID, Tenant: alarm.Tenant, Timestamp: time.Now().UTC()})
}

func (a *alarmService) GetConfiguration() Configuration {
	return *a.config
}
