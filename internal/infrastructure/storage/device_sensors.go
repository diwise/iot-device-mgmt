package storage

import (
	"context"
	"errors"

	"github.com/diwise/iot-device-mgmt/internal/application/devices"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Storage) AssignSensor(ctx context.Context, deviceID, sensorID string) error {
	if deviceID == "" {
		return ErrNoID
	}
	if sensorID == "" {
		return devices.ErrSensorNotFound
	}

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		log.Error("could not acquire connection", "err", err.Error())
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
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			switch pgErr.Code {
			case "23503":				
				return devices.ErrSensorNotFound
			case "23505":				
				return devices.ErrSensorAlreadyAssigned
			}
		}
		log.Error("could not assign sensor to device", "err", err.Error())
		return err
	}
	if result.RowsAffected() == 0 {
		return devices.ErrDeviceNotFound
	}

	return nil
}

func (s *Storage) UnassignSensor(ctx context.Context, deviceID string) error {
	if deviceID == "" {
		return ErrNoID
	}

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		log.Error("could not acquire connection", "err", err.Error())
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
		log.Error("could not unassign sensor", "err", err.Error())
		return err
	}
	if result.RowsAffected() == 0 {
		return devices.ErrDeviceNotFound
	}

	return nil
}
