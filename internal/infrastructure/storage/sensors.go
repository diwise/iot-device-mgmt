package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/application/sensors"
	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	internaltypes "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/jackc/pgx/v5"
)

func (s *Storage) QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[sensors.Sensor], error) {
	condition := &internaltypes.Condition{Offset: query.Offset, Limit: query.Limit}
	offsetLimit, offset, limit := OffsetLimit(condition, 0, 10)

	args := pgx.NamedArgs{}
	where := []string{}
	if condition.Offset != nil {
		args["offset"] = offset
	}
	if condition.Limit != nil {
		args["limit"] = limit
	}
	if query.Assigned != nil {
		if *query.Assigned {
			where = append(where, "d.device_id IS NOT NULL")
		} else {
			where = append(where, "d.device_id IS NULL")
		}
	}
	if query.HasProfile != nil {
		if *query.HasProfile {
			where = append(where, "s.sensor_profile IS NOT NULL AND s.sensor_profile <> ''")
		} else {
			where = append(where, "(s.sensor_profile IS NULL OR s.sensor_profile = '')")
		}
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[sensors.Sensor]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, fmt.Sprintf(`
		SELECT
			s.sensor_id,
			d.device_id,
			sp.name,
			sp.decoder,
			sp.interval,
			count(*) OVER () AS count
		FROM sensors s
		LEFT JOIN devices d ON d.sensor_id = s.sensor_id AND d.deleted = FALSE
		LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
		%s
		ORDER BY s.sensor_id ASC
		%s`, whereClause, offsetLimit), args)
	if err != nil {
		return types.Collection[sensors.Sensor]{}, err
	}
	defer rows.Close()

	items := []sensors.Sensor{}
	var count uint64
	for rows.Next() {
		var sensorID string
		var deviceID *string
		var profileName, decoder *string
		var interval *int

		err = rows.Scan(&sensorID, &deviceID, &profileName, &decoder, &interval, &count)
		if err != nil {
			return types.Collection[sensors.Sensor]{}, err
		}

		items = append(items, sensorFromRow(sensorID, deviceID, profileName, decoder, interval))
	}

	if err = rows.Err(); err != nil {
		return types.Collection[sensors.Sensor]{}, err
	}

	return types.Collection[sensors.Sensor]{
		Data:       items,
		Count:      uint64(len(items)),
		Offset:     uint64(offset),
		Limit:      uint64(limit),
		TotalCount: count,
	}, nil
}

func (s *Storage) GetSensor(ctx context.Context, sensorID string) (sensors.Sensor, bool, error) {
	if sensorID == "" {
		return sensors.Sensor{}, false, nil
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return sensors.Sensor{}, false, err
	}
	defer c.Release()

	var profileName, decoder *string
	var deviceID *string
	var interval *int

	err = c.QueryRow(ctx, `
		SELECT
			s.sensor_id,
			d.device_id,
			sp.name,
			sp.decoder,
			sp.interval
		FROM sensors s
		LEFT JOIN devices d ON d.sensor_id = s.sensor_id AND d.deleted = FALSE
		LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
		WHERE s.sensor_id = @sensor_id`, pgx.NamedArgs{"sensor_id": sensorID}).Scan(&sensorID, &deviceID, &profileName, &decoder, &interval)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return sensors.Sensor{}, false, nil
		}
		return sensors.Sensor{}, false, err
	}

	return sensorFromRow(sensorID, deviceID, profileName, decoder, interval), true, nil
}

func (s *Storage) CreateSensor(ctx context.Context, sensor sensors.Sensor) error {
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
		return sensors.ErrSensorAlreadyExists
	}

	return nil
}

func (s *Storage) UpdateSensor(ctx context.Context, sensor sensors.Sensor) error {
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
		return sensors.ErrSensorNotFound
	}

	return nil
}

func sensorProfileDecoder(sensor sensors.Sensor) any {
	if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder == "" {
		return nil
	}

	return strings.ToLower(strings.TrimSpace(sensor.SensorProfile.Decoder))
}

func sensorFromRow(sensorID string, deviceID, profileName, decoder *string, interval *int) sensors.Sensor {
	sensor := sensors.Sensor{SensorID: sensorID, DeviceID: deviceID}
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
