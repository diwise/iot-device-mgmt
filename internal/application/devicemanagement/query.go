package devicemanagement

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/diwise/iot-device-mgmt/internal/infrastructure/storage"
	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

var errDeviceNotFound = fmt.Errorf("device not found")
var errDeviceProfileNotFound = fmt.Errorf("device profile not found")
var errMissingTenant = fmt.Errorf("missing tenant")

func (s service) GetBySensorID(ctx context.Context, sensorID string, tenants []string) (types.Device, error) {
	d, err := s.reader.GetDeviceBySensorID(ctx, sensorID)
	if err != nil {
		if errors.Is(err, storage.ErrNoRows) {
			return types.Device{}, ErrDeviceNotFound
		}
		return types.Device{}, err
	}

	if slices.Contains(tenants, d.Tenant) {
		return d, nil
	}

	return types.Device{}, ErrDeviceNotFound
}

func (s service) GetByDeviceID(ctx context.Context, deviceID string, tenants []string) (types.Device, error) {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(deviceID), conditions.WithTenants(tenants))
	if err != nil {
		if errors.Is(err, storage.ErrNoRows) {
			return types.Device{}, ErrDeviceNotFound
		}
		return types.Device{}, err
	}

	if result.Count != 1 {
		return types.Device{}, ErrDeviceNotFound
	}

	return result.Data[0], nil
}

func (s service) GetDeviceStatus(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.SensorStatus], error) {
	if deviceID == "" {
		return types.Collection[types.SensorStatus]{}, ErrDeviceNotFound
	}

	if len(tenants) == 0 {
		return types.Collection[types.SensorStatus]{}, ErrMissingTenant
	}

	conds := conditions.Parse(ctx, params)
	conds = append(conds, conditions.WithTenants(tenants))

	return s.reader.GetDeviceStatus(ctx, deviceID, conds...)
}

func (s service) GetDeviceAlarms(ctx context.Context, deviceID string, tenants []string) (types.Collection[types.AlarmDetails], error) {
	_, err := s.GetByDeviceID(ctx, deviceID, tenants)
	if err != nil {
		return types.Collection[types.AlarmDetails]{}, err
	}

	return s.reader.GetDeviceAlarms(ctx, deviceID)
}

func (s service) Query(ctx context.Context, params map[string][]string, tenants []string) (types.Collection[types.Device], error) {
	conds := conditions.Parse(ctx, params)
	conds = append(conds, conditions.WithTenants(tenants))

	return s.reader.Query(ctx, conds...)
}

func (s service) GetTenants(ctx context.Context) (types.Collection[string], error) {
	return s.reader.GetTenants(ctx)
}

func (s service) GetLwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
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

func (s service) GetDeviceProfiles(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
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

func (s service) GetDeviceMeasurements(ctx context.Context, deviceID string, params map[string][]string, tenants []string) (types.Collection[types.Measurement], error) {
	conds := conditions.Parse(ctx, params)

	conds = append(conds, conditions.WithDeviceID(deviceID))
	conds = append(conds, conditions.WithTenants(tenants))

	return s.reader.GetDeviceMeasurements(ctx, deviceID, conds...)
}
