package devicemanagement

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
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
	Get(ctx context.Context, offset, limit int, tenants []string) (repositories.Collection[models.Device], error)
	GetBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error)
	GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error)
	GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (models.Device, error)

	Create(ctx context.Context, device models.Device) error
	Update(ctx context.Context, device models.Device) error
	Merge(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error

	UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error
	UpdateState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error

	Seed(ctx context.Context, reader io.Reader, tenants []string) error

	GetLwm2mTypes(ctx context.Context, urn ...string) (repositories.Collection[models.Lwm2mType], error)
	GetDeviceProfiles(ctx context.Context, name ...string) (repositories.Collection[models.DeviceProfile], error)
}

type svc struct {
	storage   deviceStorage.DeviceRepository
	messenger messaging.MsgContext
	config    *DeviceManagementConfig
}

func New(d deviceStorage.DeviceRepository, m messaging.MsgContext, config *DeviceManagementConfig) DeviceManagement {
	dm := svc{
		storage:   d,
		messenger: m,
		config:    config,
	}

	dm.messenger.RegisterTopicMessageHandler("device-status", NewDeviceStatusHandler(m, dm))
	dm.messenger.RegisterTopicMessageHandler("alarms.alarmCreated", NewAlarmCreatedHandler(dm))
	dm.messenger.RegisterTopicMessageHandler("alarms.alarmClosed", NewAlarmClosedHandler(dm))

	return dm
}

func (d svc) Create(ctx context.Context, device models.Device) error {
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

func (d svc) Update(ctx context.Context, device models.Device) error {
	_, err := d.storage.GetByDeviceID(ctx, device.DeviceID, []string{device.Tenant})
	if err != nil {
		return err
	}

	err = d.storage.Save(ctx, device)
	if err != nil {
		return err
	}

	return nil
}

func (d svc) Merge(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
	device, err := d.storage.GetByDeviceID(ctx, deviceID, tenants)
	if err != nil {
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

func (d svc) Get(ctx context.Context, offset, limit int, tenants []string) (repositories.Collection[models.Device], error) {
	return d.storage.Get(ctx, offset, limit, tenants)
}

var ErrDeviceNotFound = fmt.Errorf("device not found")

func (d svc) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error) {
	device, err := d.storage.GetBySensorID(ctx, sensorID, tenants)
	if err != nil {
		if errors.Is(err, deviceStorage.ErrDeviceNotFound) {
			return models.Device{}, ErrDeviceNotFound
		}
		return models.Device{}, err
	}
	return device, nil
}

func (d svc) GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (models.Device, error) {
	device, err := d.storage.GetWithAlarmID(ctx, alarmID, tenants)
	if err != nil {
		if errors.Is(err, deviceStorage.ErrDeviceNotFound) {
			return models.Device{}, ErrDeviceNotFound
		}
		return models.Device{}, err
	}
	return device, nil
}

func (d svc) GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error) {
	device, err := d.storage.GetByDeviceID(ctx, deviceID, tenants)
	if err != nil {
		if errors.Is(err, deviceStorage.ErrDeviceNotFound) {
			return models.Device{}, ErrDeviceNotFound
		}
		return models.Device{}, err
	}
	return device, nil
}

func (d svc) UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error {
	logger := logging.GetFromContext(ctx).With(slog.String("func", "UpdateDeviceStatus"))
	ctx = logging.NewContextWithLogger(ctx, logger)

	if deviceStatus.ObservedAt.IsZero() {
		logger.Debug("lastObserved is zero, set to Now")
		deviceStatus.ObservedAt = time.Now()
	}

	err := d.storage.UpdateStatus(ctx, deviceID, tenant, deviceStatus)
	if err != nil {
		return err
	}

	device, err := d.storage.GetByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	return d.messenger.PublishOnTopic(ctx, &models.DeviceStatusUpdated{
		DeviceID:  device.DeviceID,
		Tenant:    device.Tenant,
		Timestamp: deviceStatus.ObservedAt.UTC(),
	})
}

func (d svc) UpdateState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error {
	logger := logging.GetFromContext(ctx).With(slog.String("func", "UpdateDeviceState"))
	ctx = logging.NewContextWithLogger(ctx, logger)

	device, err := d.storage.GetByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	if deviceState.ObservedAt.IsZero() {
		logger.Debug("observedAt is zero, set to Now")
		deviceState.ObservedAt = time.Now().UTC()
	}

	err = d.storage.UpdateState(ctx, deviceID, tenant, deviceState)
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

var ErrDeviceProfileNotFound = fmt.Errorf("device profile not found")

func (d svc) GetDeviceProfiles(ctx context.Context, name ...string) (repositories.Collection[models.DeviceProfile], error) {
	var collection repositories.Collection[models.DeviceProfile]

	if len(name) > 0 && name[0] != "" {
		profiles := []models.DeviceProfile{}

		for _, n := range name {
			id := slices.IndexFunc(d.config.DeviceProfiles, func(p models.DeviceProfile) bool {
				return n == p.Name
			})
			if id > -1 {
				profiles = append(profiles, d.config.DeviceProfiles[id])
			}
		}

		if len(profiles) > 0 {
			collection = repositories.Collection[models.DeviceProfile]{
				Data:       profiles,
				Count:      uint64(len(profiles)),
				Offset:     0,
				Limit:      uint64(len(profiles)),
				TotalCount: uint64(len(profiles)),
			}
			return collection, nil
		}

		return repositories.Collection[models.DeviceProfile]{}, ErrDeviceProfileNotFound
	}

	collection = repositories.Collection[models.DeviceProfile]{
		Data:       d.config.DeviceProfiles,
		Count:      uint64(len(d.config.DeviceProfiles)),
		Offset:     0,
		Limit:      uint64(len(d.config.DeviceProfiles)),
		TotalCount: uint64(len(d.config.DeviceProfiles)),
	}

	return collection, nil

}

func (d svc) GetLwm2mTypes(ctx context.Context, urn ...string) (repositories.Collection[models.Lwm2mType], error) {
	var collection repositories.Collection[models.Lwm2mType]

	if len(urn) > 0 && urn[0] != "" {
		types := []models.Lwm2mType{}

		for _, u := range urn {
			id := slices.IndexFunc(d.config.Types, func(p models.Lwm2mType) bool {
				return u == p.Urn
			})
			if id > -1 {
				types = append(types, d.config.Types[id])
			}
		}

		if len(types) > 0 {
			collection = repositories.Collection[models.Lwm2mType]{
				Data:       types,
				Count:      uint64(len(types)),
				Offset:     0,
				Limit:      uint64(len(types)),
				TotalCount: uint64(len(types)),
			}
			return collection, nil
		}

		return repositories.Collection[models.Lwm2mType]{}, ErrDeviceProfileNotFound
	}

	collection = repositories.Collection[models.Lwm2mType]{
		Data:       d.config.Types,
		Count:      uint64(len(d.config.Types)),
		Offset:     0,
		Limit:      uint64(len(d.config.Types)),
		TotalCount: uint64(len(d.config.Types)),
	}

	return collection, nil
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
