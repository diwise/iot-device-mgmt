package devicemanagement

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

var errDeviceAlreadyExist = fmt.Errorf("device already exists")

func (s service) Create(ctx context.Context, device types.Device) error {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(device.DeviceID))
	if err != nil {
		return err
	}

	if result.Count > 0 {
		return ErrDeviceAlreadyExist
	}

	err = s.writer.CreateOrUpdateDevice(ctx, device)
	if err != nil {
		return err
	}

	if len(device.SensorProfile.Types) > 0 {
		l := []types.Lwm2mType{}
		for _, t := range device.SensorProfile.Types {
			l = append(l, types.Lwm2mType{
				Urn:  t,
				Name: t,
			})
		}

		s.writer.SetDeviceProfileTypes(ctx, device.DeviceID, l)
	}

	return nil
}

func (s service) Update(ctx context.Context, device types.Device) error {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(device.DeviceID))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	err = s.writer.CreateOrUpdateDevice(ctx, device)
	if err != nil {
		return err
	}

	return nil
}

func (s service) Merge(ctx context.Context, deviceID string, fields map[string]any, tenants []string) error {
	log := logging.GetFromContext(ctx)

	result, err := s.reader.Query(ctx, conditions.WithDeviceID(deviceID), conditions.WithTenants(tenants))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	if result.Count > 1 {
		return fmt.Errorf("too many devices found")
	}

	var active *bool
	var name, description, environment, source, tenant, deviceProfile *string
	var location *types.Location
	var lwm2m []string
	var interval *int

	for k, v := range fields {
		switch k {
		case "deviceID":
			continue
		case "active":
			b := v.(bool)
			active = &b
		case "description":
			s := v.(string)
			description = &s
		case "latitude":
			lat := v.(float64)
			if location == nil {
				location = &types.Location{}
			}
			location.Latitude = lat
		case "longitude":
			lon := v.(float64)
			if location == nil {
				location = &types.Location{}
			}
			location.Longitude = lon
		case "name":
			s := v.(string)
			name = &s
		case "environment":
			s := v.(string)
			environment = &s
		case "source":
			s := v.(string)
			source = &s
		case "tenant":
			s := v.(string)
			tenant = &s
		case "types":
			types := v.([]any)
			for _, typ := range types {
				s := typ.(string)
				lwm2m = append(lwm2m, s)
			}
		case "deviceProfile":
			s := v.(string)
			deviceProfile = &s
		case "interval":
			s := v.(string)
			if i, err := strconv.Atoi(s); err == nil {
				interval = &i
			}
		default:
			log.Debug("field not mapped for merge", "device_id", deviceID, "name", k)
		}
	}

	err = s.writer.UpdateDevice(ctx, deviceID, active, name, description, environment, source, tenant, location, interval)
	if err != nil {
		log.Error("could not update device information", "err", err.Error())
		return err
	}

	if deviceProfile != nil {
		err = s.writer.SetSensorProfile(ctx, deviceID, types.SensorProfile{
			Decoder: *deviceProfile,
		})
		if err != nil {
			log.Error("could not set device profile for device", "device_id", deviceID, "profile", deviceProfile, "err", err.Error())
			return err
		}
	}

	if len(lwm2m) > 0 {
		l := []types.Lwm2mType{}
		for _, t := range lwm2m {
			if t == "" {
				continue
			}
			l = append(l, types.Lwm2mType{
				Urn: strings.ToLower(strings.TrimSpace(t)),
			})
		}

		err = s.writer.SetDeviceProfileTypes(ctx, deviceID, l)
		if err != nil {
			log.Error("could not set lwm2m types for device", "device_id", deviceID, "err", err.Error())
			return err
		}
	}

	return nil
}

func (s service) UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error {
	result, err := s.reader.Query(ctx, conditions.WithDeviceID(deviceID), conditions.WithTenant(tenant))
	if err != nil {
		return err
	}

	if result.Count == 0 {
		return ErrDeviceNotFound
	}

	return s.statusWriter.SetDeviceState(ctx, deviceID, deviceState)
}
