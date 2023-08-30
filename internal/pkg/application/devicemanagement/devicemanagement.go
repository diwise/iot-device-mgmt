package devicemanagement

import (
	"context"
	"time"

	r "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	t "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

//go:generate moq -rm -out devicemanagement_mock.go . DeviceManagement

type DeviceManagement interface {
	GetDevices(ctx context.Context, offset, limit uint64, tenants ...string) (uint64, []r.Device, error)
	GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (r.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (r.Device, error)

	UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus r.DeviceStatus) error
	UpdateDeviceState(ctx context.Context, deviceID string, deviceState r.DeviceState) error

	CreateDevice(ctx context.Context, device t.Device) error
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error

	AddAlarm(ctx context.Context, deviceID string, alarmID int, severity int, observedAt time.Time) error
	RemoveAlarm(ctx context.Context, alarmID int) error
}

type deviceManagement struct {
	deviceRepository r.DeviceRepository
	messenger        messaging.MsgContext
}

func New(d r.DeviceRepository, m messaging.MsgContext) DeviceManagement {
	dm := &deviceManagement{
		deviceRepository: d,
		messenger:        m,
	}

	dm.messenger.RegisterTopicMessageHandler("device-status", DeviceStatusHandler(m, dm))
	dm.messenger.RegisterTopicMessageHandler("alarms.alarmCreated", AlarmsCreatedHandler(m, dm))
	dm.messenger.RegisterTopicMessageHandler("alarms.alarmClosed", AlarmsClosedHandler(m, dm))

	return dm
}

func (d *deviceManagement) CreateDevice(ctx context.Context, device t.Device) error {
	dataModel, err := MapTo[r.Device](device)
	if err != nil {
		return err
	}

	err = d.deviceRepository.Save(ctx, &dataModel)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &t.DeviceCreated{
		DeviceID:  dataModel.DeviceID,
		Tenant:    dataModel.Tenant.Name,
		Timestamp: dataModel.CreatedAt,
	})
}

func (d *deviceManagement) UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error {
	return nil
}

func (d *deviceManagement) GetDevices(ctx context.Context, offset, limit uint64, tenants ...string) (uint64, []r.Device, error) {
	return d.deviceRepository.GetDevices(ctx, offset, limit, tenants...)
}

func (d *deviceManagement) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (r.Device, error) {
	return d.deviceRepository.GetDeviceBySensorID(ctx, sensorID, tenants...)
}

func (d *deviceManagement) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (r.Device, error) {
	return d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID, tenants...)
}

func (d *deviceManagement) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus r.DeviceStatus) error {
	logger := logging.GetFromContext(ctx).With().Str("func", "UpdateDeviceStatus").Logger()
	ctx = logging.NewContextWithLogger(ctx, logger)

	if deviceStatus.LastObserved.IsZero() {
		logger.Debug().Msgf("lastObserved is zero, set to Now")
		deviceStatus.LastObserved = time.Now().UTC()
	}

	err := d.deviceRepository.UpdateDeviceStatus(ctx, deviceID, deviceStatus)
	if err != nil {
		return err
	}

	device, err := d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &t.DeviceStatusUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant.Name,
		Timestamp: deviceStatus.LastObserved,
	})
}

func (d *deviceManagement) UpdateDeviceState(ctx context.Context, deviceID string, deviceState r.DeviceState) error {
	logger := logging.GetFromContext(ctx).With().Str("func", "UpdateDeviceState").Logger()
	ctx = logging.NewContextWithLogger(ctx, logger)

	device, err := d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	if deviceState.ObservedAt.IsZero() {
		logger.Debug().Msgf("observedAt is zero, set to Now")
		deviceState.ObservedAt = time.Now().UTC()
	}

	logger.Debug().Msgf("online: %t, state: %d", deviceState.Online, deviceState.State)

	if has, highestSeverity, _ := device.HasActiveAlarms(); has {
		switch highestSeverity {
		case 1:
			deviceState.State = r.DeviceStateOK
		case 2:
			deviceState.State = r.DeviceStateWarning
		case 3:
			deviceState.State = r.DeviceStateError
		default:
			deviceState.State = r.DeviceStateUnknown
		}

		logger.Debug().Msgf("has alarms with severity %d, state set to %d", highestSeverity, deviceState.State)
	}

	err = d.deviceRepository.UpdateDeviceState(ctx, deviceID, deviceState)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &t.DeviceStateUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant.Name,
		State:     deviceState.State,
		Timestamp: deviceState.ObservedAt,
	})
}

func (d *deviceManagement) AddAlarm(ctx context.Context, deviceID string, alarmID int, severity int, observedAt time.Time) error {
	return d.deviceRepository.AddAlarm(ctx, deviceID, alarmID, severity, observedAt)
}

func (d *deviceManagement) RemoveAlarm(ctx context.Context, alarmID int) error {
	logger := logging.GetFromContext(ctx).With().Str("func", "RemoveAlarm").Logger()
	ctx = logging.NewContextWithLogger(ctx, logger)

	deviceID, err := d.deviceRepository.RemoveAlarmByID(ctx, alarmID)
	if err != nil {
		return err
	}

	device, err := d.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	if device.HasAlarm(alarmID) {
		logger.Warn().Msg("alarm not removed!")
	}

	deviceState := device.DeviceState
	deviceState.State = r.DeviceStateOK

	return d.UpdateDeviceState(ctx, deviceID, deviceState)
}
