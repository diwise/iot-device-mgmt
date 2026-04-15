package sensors

import (
	"context"

	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func (s service) Query(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
	return s.reader.QuerySensors(ctx, query)
}

func (s service) Sensor(ctx context.Context, sensorID string) (types.Sensor, error) {
	sensor, found, err := s.reader.GetSensor(ctx, sensorID)
	if err != nil {
		return types.Sensor{}, err
	}

	if !found {
		return types.Sensor{}, ErrSensorNotFound
	}

	return sensor, nil
}

func (s service) SensorProfile(ctx context.Context, profileID string) (types.SensorProfile, error) {
	profile, found, err := s.reader.GetSensorProfile(ctx, profileID)
	if err != nil {
		return types.SensorProfile{}, err
	}

	if !found {
		return types.SensorProfile{}, ErrSensorProfileNotFound
	}

	return profile, nil
}