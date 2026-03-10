package storage

import (
	"context"
	"errors"

	"github.com/diwise/iot-device-mgmt/internal/application/devicemanagement"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Storage) AssignSensor(ctx context.Context, deviceID, sensorID string) error {
	if deviceID == "" {
		return ErrNoID
	}
	if sensorID == "" {
		return devicemanagement.ErrSensorNotFound
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	result, err := c.Exec(ctx, `
		UPDATE devices
		SET sensor_id = @sensor_id,
			modified_on = NOW()
		WHERE device_id = @device_id AND deleted = FALSE`, pgx.NamedArgs{
		"device_id": deviceID,
		"sensor_id": sensorID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23503":
				return devicemanagement.ErrSensorNotFound
			case "23505":
				return devicemanagement.ErrSensorAlreadyAssigned
			}
		}
		return err
	}
	if result.RowsAffected() == 0 {
		return devicemanagement.ErrDeviceNotFound
	}

	return nil
}

func (s *Storage) UnassignSensor(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrNoID
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	result, err := c.Exec(ctx, `
		UPDATE devices
		SET sensor_id = NULL,
			modified_on = NOW()
		WHERE device_id = @device_id AND deleted = FALSE`, pgx.NamedArgs{
		"device_id": deviceID,
	})
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return devicemanagement.ErrDeviceNotFound
	}

	return nil
}
