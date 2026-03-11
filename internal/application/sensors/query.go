package sensors

import (
	"context"

	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func (s service) Query(ctx context.Context, query sensorquery.Sensors) (types.Collection[Sensor], error) {
	return s.reader.QuerySensors(ctx, query)
}

func (s service) Sensor(ctx context.Context, sensorID string) (Sensor, error) {
	sensor, found, err := s.reader.GetSensor(ctx, sensorID)
	if err != nil {
		return Sensor{}, err
	}

	if !found {
		return Sensor{}, ErrSensorNotFound
	}

	return sensor, nil
}
