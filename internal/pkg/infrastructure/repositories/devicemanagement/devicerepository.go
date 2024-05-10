package devicemanagement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	types "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	TypeName              string = "Device"
	DeviceProfileTypeName string = "DeviceProfile"
)

const storageConfiguration string = `
serviceName: device-management
entities:
  - idPattern: ^
    type: Device
    tableName: mgmt_devices
  - idPattern: ^
    type: DeviceProfile
    tableName: mgmt_device_profiles
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
	Get(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Device], error)
	GetOnlineDevices(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Device], error)
	GetBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error)
	GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error)
	GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (models.Device, error)

	Save(ctx context.Context, device models.Device) error

	UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error
	UpdateState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error

	GetTenants(ctx context.Context) []string

	GetDeviceProfiles(ctx context.Context, name string, tenants []string) (types.Collection[models.DeviceProfile], error)
	AddDeviceProfile(ctx context.Context, d models.DeviceProfile) error
}

type Repository struct {
	storage jsonstore.JsonStorage
}

func (r Repository) Get(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Device], error) {
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

func (r Repository) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error) {
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

func (r Repository) GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error) {
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

func (r Repository) GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (models.Device, error) {
	q := fmt.Sprintf("data @> '{\"alarms\":[\"%s\"]}'", alarmID)
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

func (d Repository) UpdateStatus(ctx context.Context, deviceID string, tenant string, deviceStatus models.DeviceStatus) error {
	device, err := d.GetByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	device.DeviceStatus = deviceStatus

	return d.Save(ctx, device)
}

func (d Repository) UpdateState(ctx context.Context, deviceID string, tenant string, deviceState models.DeviceState) error {
	device, err := d.GetByDeviceID(ctx, deviceID, []string{tenant})
	if err != nil {
		return err
	}

	device.DeviceState = deviceState

	return d.Save(ctx, device)
}

func (d Repository) GetTenants(ctx context.Context) []string {
	return d.storage.GetTenants(ctx)
}

func (d Repository) AddDeviceProfile(ctx context.Context, profile models.DeviceProfile) error {
	b, err := json.Marshal(profile)
	if err != nil {
		return err
	}

	return d.storage.Store(ctx, profile.Name, DeviceProfileTypeName, b, "default")
}

func (d Repository) GetDeviceProfiles(ctx context.Context, name string, tenants []string) (types.Collection[models.DeviceProfile], error) {
	// TODO: device profiles by tenant?

	if !slices.Contains(tenants, "default") {
		tenants = append(tenants, "default")
	}

	if len(name) > 0 {
		b, err := d.storage.FindByID(ctx, name, DeviceProfileTypeName, tenants)
		if err != nil {
			return types.Collection[models.DeviceProfile]{}, err
		}
		dp, err := jsonstore.MapOne[models.DeviceProfile](b)
		if err != nil {
			return types.Collection[models.DeviceProfile]{}, err
		}
		collection := types.Collection[models.DeviceProfile]{
			Data:       []models.DeviceProfile{dp},
			Count:      1,
			Offset:     0,
			Limit:      1,
			TotalCount: 1,
		}
		return collection, nil
	}

	result, err := d.storage.FetchType(ctx, DeviceProfileTypeName, tenants)
	if err != nil {
		return types.Collection[models.DeviceProfile]{}, err
	}
	dp, err := jsonstore.MapAll[models.DeviceProfile](result.Data)
	collection := types.Collection[models.DeviceProfile]{
		Data:       dp,
		Count:      result.Count,
		Offset:     result.Offset,
		Limit:      result.Limit,
		TotalCount: result.TotalCount,
	}
	return collection, nil
}
