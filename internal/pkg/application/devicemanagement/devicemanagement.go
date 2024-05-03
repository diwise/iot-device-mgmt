package devicemanagement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"log/slog"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	deviceStorage "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/devicemanagement"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

//go:generate moq -rm -out devicemanagement_mock.go . DeviceManagement

type DeviceManagement interface {
	GetDevices(ctx context.Context, offset, limit int, tenants []string) (repositories.Collection[models.Device], error)
	GetDeviceBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error)

	UpdateDeviceStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error
	UpdateDeviceState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error

	CreateDevice(ctx context.Context, device models.Device) error
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error

	AddAlarm(ctx context.Context, deviceID string, alarmID string, severity int, observedAt time.Time) error
	RemoveAlarm(ctx context.Context, alarmID string, tenants []string) error

	Import(ctx context.Context, reader io.Reader, tenants []string) error
}

type svc struct {
	storage   deviceStorage.DeviceRepository
	messenger messaging.MsgContext
}

func New(d deviceStorage.DeviceRepository, m messaging.MsgContext) DeviceManagement {
	dm := svc{
		storage:   d,
		messenger: m,
	}

	dm.messenger.RegisterTopicMessageHandler("device-status", NewDeviceStatusHandler(m, dm))

	return dm
}

func (d svc) Import(ctx context.Context, reader io.Reader, tenants []string) error {
	return d.storage.Seed(ctx, reader, tenants)
}

func (d svc) CreateDevice(ctx context.Context, device models.Device) error {
	err := d.storage.Save(ctx, device)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &models.DeviceCreated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant,
		Timestamp: time.Now().UTC(),
	})
}

func (d svc) UpdateDevice(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
	device, err := d.storage.GetDeviceByDeviceID(ctx, deviceID, tenants)
	if err != nil{
		return err
	}

	m := make(map[string]any)
	b, err := json.Marshal(device)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &m)
	if err != nil {
		return err
	}

	for key := range m {
		if v, ok := fields[key]; ok {
			m[key] = v
		}
	}

	b, err = json.Marshal(m)
	if err != nil {
		return err
	}

	err = json.Unmarshal(b, &device)
	if err != nil {
		return err
	}

	return d.storage.Save(ctx, device)
}

func (d svc) GetDevices(ctx context.Context, offset, limit int, tenants []string) (repositories.Collection[models.Device], error) {
	return d.storage.GetDevices(ctx, offset, limit, tenants)
}

var ErrDeviceNotFound = fmt.Errorf("device not found")

func (d svc) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error) {
	device, err := d.storage.GetDeviceBySensorID(ctx, sensorID, tenants)
	if err != nil {
		if errors.Is(err, deviceStorage.ErrDeviceNotFound) {
			return models.Device{}, ErrDeviceNotFound
		}
		return models.Device{}, err
	}
	return device, nil
}

func (d svc) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error) {
	device, err := d.storage.GetDeviceByDeviceID(ctx, deviceID, tenants)
	if err != nil {
		if errors.Is(err, deviceStorage.ErrDeviceNotFound) {
			return models.Device{}, ErrDeviceNotFound
		}
		return models.Device{}, err
	}
	return device, nil
}

func (d svc) UpdateDeviceStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error {
	logger := logging.GetFromContext(ctx).With(slog.String("func", "UpdateDeviceStatus"))
	ctx = logging.NewContextWithLogger(ctx, logger)

	if deviceStatus.ObservedAt.IsZero() {
		logger.Debug("lastObserved is zero, set to Now")
		deviceStatus.ObservedAt = time.Now()
	}

	err := d.storage.UpdateDeviceStatus(ctx, deviceID, tenant, deviceStatus)
	if err != nil {
		return err
	}

	device, err := d.storage.GetDeviceByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &models.DeviceStatusUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant,
		Timestamp: deviceStatus.ObservedAt.UTC(),
	})
}

func (d svc) UpdateDeviceState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error {
	logger := logging.GetFromContext(ctx).With(slog.String("func", "UpdateDeviceState"))
	ctx = logging.NewContextWithLogger(ctx, logger)

	device, err := d.storage.GetDeviceByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	if deviceState.ObservedAt.IsZero() {
		logger.Debug("observedAt is zero, set to Now")
		deviceState.ObservedAt = time.Now().UTC()
	}
	/*
		logger.Debug(fmt.Sprintf("online: %t, state: %d", deviceState.Online, deviceState.State))

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

			logger.Debug(fmt.Sprintf("has alarms with severity %d, state set to %d", highestSeverity, deviceState.State))
		}
	*/
	err = d.storage.UpdateDeviceState(ctx, deviceID, tenant, deviceState)
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &models.DeviceStateUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant,
		State:     deviceState.State,
		Timestamp: deviceState.ObservedAt,
	})
}

func (d svc) AddAlarm(ctx context.Context, deviceID string, alarmID string, severity int, observedAt time.Time) error {
	return d.storage.AddAlarm(ctx, deviceID, alarmID, severity, observedAt)
}

func (d svc) RemoveAlarm(ctx context.Context, alarmID string, tenants []string) error {
	logger := logging.GetFromContext(ctx).With(slog.String("func", "RemoveAlarm"))
	ctx = logging.NewContextWithLogger(ctx, logger)

	deviceID, err := d.storage.RemoveAlarmByID(ctx, alarmID)
	if err != nil {
		return err
	}

	device, err := d.storage.GetDeviceByDeviceID(ctx, deviceID, tenants)
	if err != nil {
		return err
	}
	/*
		if device.HasAlarm(alarmID) {
			logger.Warn("alarm not removed!")
		}
	*/
	deviceState := device.DeviceState
	deviceState.State = models.DeviceStateOK

	return d.UpdateDeviceState(ctx, deviceID, device.Tenant, deviceState)
}

func MapOne[T any](v any) (T, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return *new(T), err
	}
	to := new(T)
	err = json.Unmarshal(b, to)
	if err != nil {
		return *new(T), err
	}
	return *to, nil
}

func MapAll[T any](arr []any) ([]T, error) {
	result := *new([]T)

	for _, v := range arr {
		to, err := MapOne[T](v)
		if err != nil {
			return *new([]T), err
		}
		result = append(result, to)
	}

	return result, nil
}
