package devicemanagement

import (
	"context"
	"strings"

	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devicemanagement/query"
)

func (s service) AttachSensor(ctx context.Context, deviceID, sensorID string, tenants []string) error {
	result, err := s.reader.Query(ctx, dmquery.Devices{Filters: dmquery.Filters{DeviceID: deviceID, AllowedTenants: tenants}})
	if err != nil {
		return err
	}
	if result.Count != 1 {
		return ErrDeviceNotFound
	}

	sensorID = strings.TrimSpace(sensorID)
	if sensorID == "" {
		return ErrSensorNotFound
	}

	err = s.ensureSensorCanBeAssigned(ctx, deviceID, sensorID)
	if err != nil {
		return err
	}

	return s.writer.AssignSensor(ctx, deviceID, sensorID)
}

func (s service) DetachSensor(ctx context.Context, deviceID string, tenants []string) error {
	result, err := s.reader.Query(ctx, dmquery.Devices{Filters: dmquery.Filters{DeviceID: deviceID, AllowedTenants: tenants}})
	if err != nil {
		return err
	}
	if result.Count != 1 {
		return ErrDeviceNotFound
	}

	return s.writer.UnassignSensor(ctx, deviceID)
}
