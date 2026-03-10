package devicemanagement

import (
	"context"
	"fmt"
	"slices"

	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devicemanagement/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

var errDeviceNotFound = fmt.Errorf("device not found")
var errDeviceProfileNotFound = fmt.Errorf("device profile not found")
var errMissingTenant = fmt.Errorf("missing tenant")

func (s service) DeviceBySensor(ctx context.Context, sensorID string, tenants []string) (types.Device, error) {
	d, found, err := s.reader.GetDeviceBySensorID(ctx, sensorID)
	if err != nil {
		return types.Device{}, err
	}

	if !found {
		return types.Device{}, ErrDeviceNotFound
	}

	if slices.Contains(tenants, d.Tenant) {
		return d, nil
	}

	return types.Device{}, ErrDeviceNotFound
}

func (s service) Device(ctx context.Context, deviceID string, tenants []string) (types.Device, error) {
	result, err := s.reader.Query(ctx, dmquery.Devices{Filters: dmquery.Filters{
		DeviceID:       deviceID,
		AllowedTenants: tenants,
	}})
	if err != nil {
		return types.Device{}, err
	}

	if result.Count != 1 {
		return types.Device{}, ErrDeviceNotFound
	}

	return result.Data[0], nil
}

func (s service) Status(ctx context.Context, deviceID string, query dmquery.Status) (types.Collection[types.SensorStatus], error) {
	if deviceID == "" {
		return types.Collection[types.SensorStatus]{}, ErrDeviceNotFound
	}

	if len(query.AllowedTenants) == 0 {
		return types.Collection[types.SensorStatus]{}, ErrMissingTenant
	}

	return s.reader.GetDeviceStatus(ctx, deviceID, query)
}

func (s service) Alarms(ctx context.Context, deviceID string, tenants []string) (types.Collection[types.AlarmDetails], error) {
	_, err := s.Device(ctx, deviceID, tenants)
	if err != nil {
		return types.Collection[types.AlarmDetails]{}, err
	}

	return s.reader.GetDeviceAlarms(ctx, deviceID)
}

func (s service) Query(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
	return s.reader.Query(ctx, query)
}

func (s service) Tenants(ctx context.Context) (types.Collection[string], error) {
	return s.reader.GetTenants(ctx)
}

func (s service) Lwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
	var collection types.Collection[types.Lwm2mType]

	if len(urn) > 0 && urn[0] != "" {
		lwm2mTypes := []types.Lwm2mType{}

		for _, u := range urn {
			id := slices.IndexFunc(s.config.Types, func(p types.Lwm2mType) bool {
				return u == p.Urn
			})
			if id > -1 {
				lwm2mTypes = append(lwm2mTypes, s.config.Types[id])
			}
		}

		if len(lwm2mTypes) > 0 {
			collection = types.Collection[types.Lwm2mType]{
				Data:       lwm2mTypes,
				Count:      uint64(len(lwm2mTypes)),
				Offset:     0,
				Limit:      uint64(len(lwm2mTypes)),
				TotalCount: uint64(len(lwm2mTypes)),
			}
			return collection, nil
		}

		return types.Collection[types.Lwm2mType]{}, ErrDeviceProfileNotFound
	}

	collection = types.Collection[types.Lwm2mType]{
		Data:       s.config.Types,
		Count:      uint64(len(s.config.Types)),
		Offset:     0,
		Limit:      uint64(len(s.config.Types)),
		TotalCount: uint64(len(s.config.Types)),
	}

	return collection, nil
}

func (s service) Profiles(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
	var collection types.Collection[types.SensorProfile]

	if len(name) > 0 && name[0] != "" {
		profiles := []types.SensorProfile{}

		for _, n := range name {
			id := slices.IndexFunc(s.config.DeviceProfiles, func(p types.SensorProfile) bool {
				return n == p.Name
			})
			if id > -1 {
				profiles = append(profiles, s.config.DeviceProfiles[id])
			}
		}

		if len(profiles) > 0 {
			collection = types.Collection[types.SensorProfile]{
				Data:       profiles,
				Count:      uint64(len(profiles)),
				Offset:     0,
				Limit:      uint64(len(profiles)),
				TotalCount: uint64(len(profiles)),
			}
			return collection, nil
		}

		return types.Collection[types.SensorProfile]{}, ErrDeviceProfileNotFound
	}

	collection = types.Collection[types.SensorProfile]{
		Data:       s.config.DeviceProfiles,
		Count:      uint64(len(s.config.DeviceProfiles)),
		Offset:     0,
		Limit:      uint64(len(s.config.DeviceProfiles)),
		TotalCount: uint64(len(s.config.DeviceProfiles)),
	}

	return collection, nil
}

func (s service) Measurements(ctx context.Context, deviceID string, query dmquery.Measurements) (types.Collection[types.Measurement], error) {
	if len(query.AllowedTenants) == 0 {
		return types.Collection[types.Measurement]{}, ErrMissingTenant
	}

	return s.reader.GetDeviceMeasurements(ctx, deviceID, query)
}
