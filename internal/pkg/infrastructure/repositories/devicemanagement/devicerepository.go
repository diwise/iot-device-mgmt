package devicemanagement

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	types "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	TypeName string = "Device"
)

const storageConfiguration string = `
serviceName: device-management
entities:
  - idPattern: ^
    type: Device
    tableName: mgmt_devices
`

func NewRepository(ctx context.Context, p *pgxpool.Pool) (Repository, error) {
	r := bytes.NewBuffer([]byte(storageConfiguration))
	store, err := jsonstore.NewWithPool(ctx, p, r)
	if err != nil {
		return Repository{}, err
	}

	err = store.Initialize(ctx, "CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_sensor_id_active ON mgmt_devices ((data ->> 'sensor_id'), (data ->> 'active'));")
	if err != nil {
		return Repository{}, err
	}

	return Repository{
		storage: store,
	}, nil
}

//go:generate moq -rm -out devicerepository_mock.go .

type DeviceRepository interface {
	Get(ctx context.Context, offset, limit int, q string, sortBy string, tenants []string) (types.Collection[models.Device], error)
	GetOnlineDevices(ctx context.Context, offset, limit int, sortBy string, tenants []string) (types.Collection[models.Device], error)
	GetBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error)
	GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error)
	GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (models.Device, error)

	Save(ctx context.Context, device models.Device) error

	UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error
	UpdateState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error

	GetTenants(ctx context.Context) []string
}

type Repository struct {
	storage jsonstore.JsonStorage
}

func (r Repository) Get(ctx context.Context, offset, limit int, q string, sortBy string, tenants []string) (types.Collection[models.Device], error) {
	var result jsonstore.QueryResult
	var err error

	if q != "" {
		query := fmt.Sprintf("data @> '%s'", q)
		result, err = r.storage.QueryType(ctx, TypeName, query, tenants, jsonstore.Offset(offset), jsonstore.Limit(limit), jsonstore.SortBy(sortBy))
	} else {
		result, err = r.storage.FetchType(ctx, TypeName, tenants, jsonstore.Offset(offset), jsonstore.Limit(limit), jsonstore.SortBy(sortBy))
	}

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

func (r Repository) GetOnlineDevices(ctx context.Context, offset, limit int, sortBy string, tenants []string) (types.Collection[models.Device], error) {
	return r.Get(ctx, offset,limit, "'{\"deviceState\":{\"online\": true}}", sortBy, tenants)
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
		for i := range result.Data {
			d, err := jsonstore.MapOne[models.Device](result.Data[i])
			if err != nil {
				continue
			}
			if d.Active {
				return d, nil
			}
		}

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
	q := fmt.Sprintf("{\"alarms\":[\"%s\"]}", alarmID)

	result, err :=  r.Get(ctx, 0, 100, q, "", tenants)		
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

	return result.Data[0], nil
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
