package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	sensormanagement "github.com/diwise/iot-device-mgmt/internal/application/sensormanagement"
	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensormanagement/query"
	internaltypes "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5"
)

func (s *Storage) QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[sensormanagement.Sensor], error) {
	condition := &internaltypes.Condition{Offset: query.Offset, Limit: query.Limit}
	offsetLimit, offset, limit := OffsetLimit(condition, 0, 10)

	args := pgx.NamedArgs{}
	if condition.Offset != nil {
		args["offset"] = offset
	}
	if condition.Limit != nil {
		args["limit"] = limit
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[sensormanagement.Sensor]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, fmt.Sprintf(`
		SELECT
			s.sensor_id,
			sp.name,
			sp.decoder,
			sp.interval,
			count(*) OVER () AS count
		FROM sensors s
		LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
		ORDER BY s.sensor_id ASC
		%s`, offsetLimit), args)
	if err != nil {
		return types.Collection[sensormanagement.Sensor]{}, err
	}
	defer rows.Close()

	items := []sensormanagement.Sensor{}
	var count uint64
	for rows.Next() {
		var sensorID string
		var profileName, decoder *string
		var interval *int

		err = rows.Scan(&sensorID, &profileName, &decoder, &interval, &count)
		if err != nil {
			return types.Collection[sensormanagement.Sensor]{}, err
		}

		items = append(items, sensorFromRow(sensorID, profileName, decoder, interval))
	}

	if err = rows.Err(); err != nil {
		return types.Collection[sensormanagement.Sensor]{}, err
	}

	return types.Collection[sensormanagement.Sensor]{
		Data:       items,
		Count:      uint64(len(items)),
		Offset:     uint64(offset),
		Limit:      uint64(limit),
		TotalCount: count,
	}, nil
}

func (s *Storage) GetSensor(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
	if sensorID == "" {
		return sensormanagement.Sensor{}, false, nil
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return sensormanagement.Sensor{}, false, err
	}
	defer c.Release()

	var profileName, decoder *string
	var interval *int

	err = c.QueryRow(ctx, `
		SELECT
			s.sensor_id,
			sp.name,
			sp.decoder,
			sp.interval
		FROM sensors s
		LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
		WHERE s.sensor_id = @sensor_id`, pgx.NamedArgs{"sensor_id": sensorID}).Scan(&sensorID, &profileName, &decoder, &interval)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sensormanagement.Sensor{}, false, nil
		}
		return sensormanagement.Sensor{}, false, err
	}

	return sensorFromRow(sensorID, profileName, decoder, interval), true, nil
}

func (s *Storage) CreateSensor(ctx context.Context, sensor sensormanagement.Sensor) error {
	args := pgx.NamedArgs{
		"sensor_id":      strings.TrimSpace(sensor.SensorID),
		"sensor_profile": sensorProfileDecoder(sensor),
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	result, err := c.Exec(ctx, `
		INSERT INTO sensors (sensor_id, sensor_profile)
		VALUES (@sensor_id, @sensor_profile)
		ON CONFLICT DO NOTHING`, args)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return sensormanagement.ErrSensorAlreadyExists
	}

	return nil
}

func (s *Storage) UpdateSensor(ctx context.Context, sensor sensormanagement.Sensor) error {
	args := pgx.NamedArgs{
		"sensor_id":      strings.TrimSpace(sensor.SensorID),
		"sensor_profile": sensorProfileDecoder(sensor),
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	result, err := c.Exec(ctx, `
		UPDATE sensors
		SET sensor_profile = @sensor_profile,
			modified_on = NOW()
		WHERE sensor_id = @sensor_id`, args)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return sensormanagement.ErrSensorNotFound
	}

	return nil
}

func sensorProfileDecoder(sensor sensormanagement.Sensor) any {
	if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder == "" {
		return nil
	}

	return strings.ToLower(strings.TrimSpace(sensor.SensorProfile.Decoder))
}

func sensorFromRow(sensorID string, profileName, decoder *string, interval *int) sensormanagement.Sensor {
	sensor := sensormanagement.Sensor{SensorID: sensorID}
	if decoder == nil {
		return sensor
	}

	profile := &types.SensorProfile{Decoder: *decoder}
	if profileName != nil {
		profile.Name = *profileName
	}
	if interval != nil {
		profile.Interval = *interval
	}

	sensor.SensorProfile = profile
	return sensor
}
