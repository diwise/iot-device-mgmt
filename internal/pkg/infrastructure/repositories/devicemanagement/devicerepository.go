package devicemanagement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	types "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

const TypeName string = "Device"

const storageConfiguration string = `
serviceName: device-management
entities:
  - idPattern: ^
    type: Device
    tableName: devices
  - idPattern: ^
    type: DeviceModel
    tableName: deviceModels
`

func NewRepository(ctx context.Context, p *pgxpool.Pool) (Repository, error) {
	r := bytes.NewBuffer([]byte(storageConfiguration))
	store, err := jsonstore.NewWithPool(ctx, p, r)
	if err != nil {
		return Repository{}, err
	}

	err = store.Initialize(ctx)
	if err != nil {
		return Repository{}, err
	}

	return Repository{
		storage: store,
	}, nil
}

//go:generate moq -rm -out devicerepository_mock.go .

type DeviceRepository interface {
	GetDevices(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Device], error)
	GetOnlineDevices(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Device], error)
	GetDeviceBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error)

	Save(ctx context.Context, device models.Device) error

	UpdateDeviceStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error
	UpdateDeviceState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error

	GetTenants(ctx context.Context) []string

	AddAlarm(ctx context.Context, deviceID string, alarmID string, severity int, observedAt time.Time) error
	RemoveAlarmByID(ctx context.Context, alarmID string) (string, error)

	Seed(ctx context.Context, csvReader io.Reader, tenants []string) error
}

type Repository struct {
	storage jsonstore.JsonStorage
}

func (r Repository) GetDevices(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Device], error) {
	result, err := r.storage.FetchType(ctx, TypeName, tenants, jsonstore.Offset(offset), jsonstore.Limit(limit))
	if err != nil {
		return types.Collection[models.Device]{}, err
	}
	if result.Count == 0 {
		return types.Collection[models.Device]{}, nil
	}

	devices, err := jsonstore.MapAll[models.Device](result.Data)
	if err != nil {
		return types.Collection[models.Device]{}, err
	}

	return types.Collection[models.Device]{
		Data:       devices,
		Count:      result.Count,
		Offset:     result.Offset,
		Limit:      result.Limit,
		TotalCount: result.TotalCount,
	}, nil
}

func (r Repository) GetOnlineDevices(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Device], error) {
	result, err := r.storage.QueryType(ctx, TypeName, "data @> '{\"deviceState\":{\"online\": true}}'", tenants, jsonstore.Offset(offset), jsonstore.Limit(limit))
	if err != nil {
		return types.Collection[models.Device]{}, err
	}
	if result.Count == 0 {
		return types.Collection[models.Device]{}, nil
	}

	devices, err := jsonstore.MapAll[models.Device](result.Data)
	if err != nil {
		return types.Collection[models.Device]{}, err
	}

	return types.Collection[models.Device]{
		Data:       devices,
		Count:      result.Count,
		Offset:     result.Offset,
		Limit:      result.Limit,
		TotalCount: result.TotalCount,
	}, nil
}

func (r Repository) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error) {
	q := fmt.Sprintf("data ->> 'sensorID' = '%s'", sensorID)
	result, err := r.storage.QueryType(ctx, TypeName, q, tenants)
	if err != nil {
		if errors.Is(err, jsonstore.ErrNoRows) {
			return models.Device{}, ErrDeviceNotFound
		}
		return models.Device{}, err
	}
	if result.Count == 0 {
		return models.Device{}, ErrDeviceNotFound
	}
	if result.Count > 1 {
		return models.Device{}, fmt.Errorf("too many devices found")
	}

	return jsonstore.MapOne[models.Device](result.Data[0])
}

var ErrDeviceNotFound = fmt.Errorf("device not found")

func (r Repository) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error) {
	result, err := r.storage.FindByID(ctx, deviceID, TypeName, tenants)
	if err != nil {
		if errors.Is(err, jsonstore.ErrNoRows) {
			return models.Device{}, ErrDeviceNotFound
		}
		return models.Device{}, err
	}
	if result == nil {
		return models.Device{}, ErrDeviceNotFound
	}

	return jsonstore.MapOne[models.Device](result)
}

func (r Repository) Save(ctx context.Context, device models.Device) error {
	b, err := json.Marshal(device)
	if err != nil {
		return err
	}

	err = r.storage.Store(ctx, device.DeviceID, TypeName, b, device.Tenant)
	if err != nil {
		return err
	}

	return nil
}

func (d Repository) UpdateDeviceStatus(ctx context.Context, deviceID string, tenant string, deviceStatus models.DeviceStatus) error {
	device, err := d.GetDeviceByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	device.DeviceStatus = deviceStatus

	return d.Save(ctx, device)
}

func (d Repository) UpdateDeviceState(ctx context.Context, deviceID string, tenant string, deviceState models.DeviceState) error {
	device, err := d.GetDeviceByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	device.DeviceState = deviceState

	return d.Save(ctx, device)
}

func (d Repository) GetTenants(ctx context.Context) []string {
	return d.storage.GetTenants(ctx)
}

func (d Repository) AddAlarm(ctx context.Context, deviceID string, alarmID string, severity int, observedAt time.Time) error {
	return fmt.Errorf("not implemented")
}

func (d Repository) RemoveAlarmByID(ctx context.Context, alarmID string) (string, error) {
	return "", fmt.Errorf("not implemented")
}