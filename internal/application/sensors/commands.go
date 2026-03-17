package sensors

import (
	"context"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func (s service) Create(ctx context.Context, sensor types.Sensor) error {
	_, found, err := s.reader.GetSensor(ctx, sensor.SensorID)
	if err != nil {
		return err
	}

	if found {
		return ErrSensorAlreadyExists
	}

	err = s.writer.CreateSensor(ctx, sensor)
	if err != nil {
		return err
	}

	return nil
}

func (s service) Update(ctx context.Context, sensor types.Sensor) error {
	_, found, err := s.reader.GetSensor(ctx, sensor.SensorID)
	if err != nil {
		return err
	}

	if !found {
		return ErrSensorNotFound
	}

	err = s.writer.UpdateSensor(ctx, sensor)
	if err != nil {
		return err
	}

	return nil
}
