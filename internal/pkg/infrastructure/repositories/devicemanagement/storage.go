package devicemanagement

import (
	"context"
	"errors"
	"strconv"
	"strings"

	types "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
)

func NewDeviceStorage(s *storage.Storage) DeviceRepository {
	return &devices{
		Storage: s,
	}
}

type devices struct {
	Storage *storage.Storage
}

func (ds *devices) Get(ctx context.Context, offset, limit int, q string, sortBy string, tenants []string) (types.Collection[models.Device], error) {
	return types.Collection[models.Device]{}, errors.New("not implemented")
}
func (ds *devices) Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[models.Device], error) {
	conditions := make([]storage.ConditionFunc, 0)

	conditions = append(conditions, storage.WithTenants(tenants))
	
	for k, v := range params {
		switch strings.ToLower(k) {
		case "deveui":
			conditions = append(conditions, storage.WithSensorID(v[0]))
		case "device_id":
			conditions = append(conditions, storage.WithDeviceID(v[0]))
		case "sensor_id":
			conditions = append(conditions, storage.WithSensorID(v[0]))
		case "type":
			conditions = append(conditions, storage.WithTypes(v))
		case "types":
			conditions = append(conditions, storage.WithTypes(v))
		case "active":
			active, _ := strconv.ParseBool(v[0])
			conditions = append(conditions, storage.WithActive(active))
		case "online":
			online, _ := strconv.ParseBool(v[0])
			conditions = append(conditions, storage.WithOnline(online))
		case "limit":
			limit, _ := strconv.Atoi(v[0])
			conditions = append(conditions, storage.WithLimit(limit))
		case "offset":
			offset, _ := strconv.Atoi(v[0])
			conditions = append(conditions, storage.WithOffset(offset))
		case "sortby":
			conditions = append(conditions, storage.WithSortBy(v[0]))
		case "sortorder":
			conditions = append(conditions, storage.WithSortDesc(strings.EqualFold(v[0], "desc")))
		}

	}
	return ds.Storage.QueryDevices(ctx, conditions...)
}

func (ds *devices) GetOnlineDevices(ctx context.Context, offset, limit int, sortBy string, tenants []string) (types.Collection[models.Device], error) {
	return ds.Storage.QueryDevices(ctx,
		storage.WithOnline(true),
		storage.WithLimit(limit),
		storage.WithOffset(offset),
		storage.WithTenants(tenants),
		storage.WithSortBy(sortBy),
	)
}

func (ds *devices) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (models.Device, error) {
	return ds.Storage.GetDevice(ctx, storage.WithSensorID(sensorID), storage.WithTenants(tenants))
}

func (ds *devices) GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (models.Device, error) {
	return ds.Storage.GetDevice(ctx, storage.WithDeviceID(deviceID), storage.WithTenants(tenants))
}

func (ds *devices) GetWithAlarmID(ctx context.Context, alarmID string, tenants []string) (models.Device, error) {
	return models.Device{}, errors.New("not implemented")
}

func (ds *devices) GetWithinBounds(ctx context.Context, b types.Bounds) (types.Collection[models.Device], error) {
	//TODO: are the coordinates in the correct order?
	return ds.Storage.QueryDevices(ctx, storage.WithBounds(b.MaxLat, b.MinLat, b.MaxLon, b.MinLon))
}

func (ds *devices) Save(ctx context.Context, device models.Device) error {
	err := ds.Storage.AddDevice(ctx, device)
	if err != nil {
		return ds.Storage.UpdateDevice(ctx, device)
	}
	return nil
}

func (ds *devices) UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus models.DeviceStatus) error {
	return ds.Storage.UpdateStatus(ctx, deviceID, tenant, deviceStatus)
}

func (ds *devices) UpdateState(ctx context.Context, deviceID, tenant string, deviceState models.DeviceState) error {
	return ds.Storage.UpdateState(ctx, deviceID, tenant, deviceState)
}

func (ds *devices) GetTenants(ctx context.Context) []string {
	return ds.Storage.GetTenants(ctx)
}
