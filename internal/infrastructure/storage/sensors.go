package storage

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/application/sensors"
	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Storage) QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
	condition := &Condition{Offset: query.Offset, Limit: query.Limit}
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
	if profileName := strings.ToLower(strings.TrimSpace(query.ProfileName)); profileName != "" {
		args["decoder"] = profileName
		where = append(where, "LOWER(sp.decoder) = @decoder")
	}
	if profileTypes := normalizeSensorProfileTypes(query.Types); len(profileTypes) > 0 {
		args["profile_types"] = profileTypes
		where = append(where, `EXISTS (
			SELECT 1
			FROM sensor_profiles_sensor_profile_types sppt
			WHERE sppt.sensor_profile_id = s.sensor_profile
				AND LOWER(sppt.sensor_profile_type_id) = ANY(@profile_types)
		)`)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		args["search"] = "%" + search + "%"
		where = append(where, "(s.sensor_id ILIKE @search OR s.name ILIKE @search)")
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		log.Error("could not acquire connection", "err", err.Error())
		return types.Collection[types.Sensor]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, fmt.Sprintf(`
		SELECT
			s.sensor_id,
			d.device_id,
			s.name,
			s.location,
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
		log.Error("failed to query sensors", "args", args, "err", err.Error())
		return types.Collection[types.Sensor]{}, err
	}
	defer rows.Close()

	items := []types.Sensor{}
	var count uint64
	for rows.Next() {
		var sensorID string
		var deviceID *string
		var name *string
		var location pgtype.Point
		var profileName, decoder *string
		var interval *int

		err = rows.Scan(&sensorID, &deviceID, &name, &location, &profileName, &decoder, &interval, &count)
		if err != nil {
			log.Error("failed to scan sensor row", "err", err.Error())
			return types.Collection[types.Sensor]{}, err
		}

		items = append(items, sensorFromRow(sensorID, deviceID, name, location, profileName, decoder, interval))
	}

	if err = rows.Err(); err != nil {
		return types.Collection[types.Sensor]{}, err
	}

	return types.Collection[types.Sensor]{
		Data:       items,
		Count:      uint64(len(items)),
		Offset:     uint64(offset),
		Limit:      uint64(limit),
		TotalCount: count,
	}, nil
}

func (s *Storage) GetSensor(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
	if sensorID == "" {
		return types.Sensor{}, false, nil
	}

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		log.Error("could not acquire connection", "err", err.Error())
		return types.Sensor{}, false, err
	}
	defer c.Release()

	var profileName, decoder *string
	var deviceID *string
	var name *string
	var location pgtype.Point
	var interval *int
	var batteryLevel *int
	var rssi, snr, sf *float64
	var fq *int64
	var dr *int
	var statusObservedAt *time.Time

	err = c.QueryRow(ctx, `
		WITH latest_status AS (
			SELECT DISTINCT ON (sensor_id)
				sensor_id, battery_level, rssi, snr, fq, sf, dr, observed_at
			FROM sensor_status
			ORDER BY sensor_id, observed_at DESC
		)

		SELECT
			s.sensor_id,
			d.device_id,
			s.name,
			s.location,
			sp.name,
			sp.decoder,
			sp.interval,

			ls.battery_level,
			ls.rssi,
			ls.snr,
			ls.fq,
			ls.sf,
			ls.dr,
			ls.observed_at  AS status_observed_at

		FROM sensors s
		LEFT JOIN devices d ON d.sensor_id = s.sensor_id AND d.deleted = FALSE
		LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
		LEFT JOIN latest_status ls ON ls.sensor_id = s.sensor_id
		WHERE s.sensor_id = @sensor_id`, pgx.NamedArgs{"sensor_id": sensorID}).Scan(&sensorID, &deviceID, &name, &location, &profileName, &decoder, &interval, &batteryLevel, &rssi, &snr, &fq, &sf, &dr, &statusObservedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Sensor{}, false, nil
		}
		log.Error("failed to query sensor", "sensor_id", sensorID, "err", err.Error())
		return types.Sensor{}, false, err
	}

	sens := sensorFromRow(sensorID, deviceID, name, location, profileName, decoder, interval)

	if statusObservedAt != nil {
		sens.SensorStatus = &types.SensorStatus{
			RSSI:            rssi,
			LoRaSNR:         snr,
			Frequency:       fq,
			SpreadingFactor: sf,
			DR:              dr,
			ObservedAt:      statusObservedAt.UTC(),
		}
		if batteryLevel != nil {
			bat := *batteryLevel
			sens.SensorStatus.BatteryLevel = int(bat)
		}
	}

	return sens, true, nil
}

func (s *Storage) CreateSensor(ctx context.Context, sensor types.Sensor) error {
	args := pgx.NamedArgs{
		"sensor_id":      strings.TrimSpace(sensor.SensorID),
		"name":           sensorName(sensor.Name),
		"location":       sensorPoint(sensor.Location),
		"sensor_profile": sensorProfileDecoder(sensor),
	}

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		log.Error("could not acquire connection", "err", err.Error())
		return err
	}
	defer c.Release()

	result, err := c.Exec(ctx, `
		INSERT INTO sensors (sensor_id, name, location, sensor_profile)
		VALUES (@sensor_id, @name, @location, @sensor_profile)
		ON CONFLICT DO NOTHING`, args)
	if err != nil {
		log.Error("could not insert sensor", "args", args, "err", err.Error())
		return err
	}

	if result.RowsAffected() == 0 {
		return sensors.ErrSensorAlreadyExists
	}

	return nil
}

func (s *Storage) UpdateSensor(ctx context.Context, sensor types.Sensor) error {
	args := pgx.NamedArgs{
		"sensor_id":      strings.TrimSpace(sensor.SensorID),
		"name":           sensorName(sensor.Name),
		"location":       sensorPoint(sensor.Location),
		"sensor_profile": sensorProfileDecoder(sensor),
	}

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		log.Error("could not acquire connection", "err", err.Error())
		return err
	}
	defer c.Release()

	result, err := c.Exec(ctx, `
		UPDATE sensors
		SET name = @name,
			location = @location,
			sensor_profile = @sensor_profile,
			modified_on = NOW()
		WHERE sensor_id = @sensor_id`, args)
	if err != nil {
		log.Error("could not update sensor", "args", args, "err", err.Error())
		return err
	}

	if result.RowsAffected() == 0 {
		return sensors.ErrSensorNotFound
	}

	return nil
}

func (s *Storage) GetSensorProfile(ctx context.Context, profileID string) (types.SensorProfile, bool, error) {
	if profileID == "" {
		return types.SensorProfile{}, false, nil
	}

	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		log.Error("could not acquire connection", "err", err.Error())
		return types.SensorProfile{}, false, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT sp.name, sp.decoder, sp.description, sp.interval, spspt.sensor_profile_type_id as urn, spt.name as type_name
		FROM sensor_profiles sp
		JOIN sensor_profiles_sensor_profile_types spspt ON sp.sensor_profile_id = spspt.sensor_profile_id
		JOIN sensor_profile_types spt ON spt.sensor_profile_type_id = spspt.sensor_profile_type_id
		WHERE sp.sensor_profile_id = @sensor_profile_id`, pgx.NamedArgs{"sensor_profile_id": profileID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.SensorProfile{}, false, nil
		}
		log.Error("failed to query sensor profile", "profile_id", profileID, "err", err.Error())
		return types.SensorProfile{}, false, err
	}
	defer rows.Close()

	profile := types.SensorProfile{}
	profileTypes := []string{}

	var name, decoder, description *string
	var interval *int

	for rows.Next() {
		var urn string
		var typeName string

		err = rows.Scan(&name, &decoder, &description, &interval, &urn, &typeName)
		if err != nil {
			log.Error("failed to scan sensor profile row", "err", err.Error())
			return types.SensorProfile{}, false, err
		}

		profileTypes = append(profileTypes, urn)
	}

	if err = rows.Err(); err != nil {
		return types.SensorProfile{}, false, err
	}

	if name == nil || decoder == nil {
		log.Error("incomplete sensor profile data", "profile_id", profileID)
		return types.SensorProfile{}, false, errors.New("incomplete sensor profile data")
	}

	profile.Name = *name
	profile.Decoder = *decoder

	if interval != nil {
		profile.Interval = *interval
	}
	profile.Types = profileTypes

	return profile, true, nil
}

func sensorProfileDecoder(sensor types.Sensor) any {
	if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder == "" {
		return nil
	}

	return strings.ToLower(strings.TrimSpace(sensor.SensorProfile.Decoder))
}

func sensorName(name *string) any {
	if name == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return nil
	}

	return trimmed
}

func sensorPoint(location *types.Location) any {
	if location == nil {
		return nil
	}

	return pgtype.Point{
		P:     pgtype.Vec2{X: location.Longitude, Y: location.Latitude},
		Valid: true,
	}
}

func normalizeSensorProfileTypes(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" || slices.Contains(normalized, trimmed) {
			continue
		}
		normalized = append(normalized, trimmed)
	}

	if len(normalized) == 0 {
		return nil
	}

	return normalized
}

func sensorFromRow(sensorID string, deviceID, name *string, location pgtype.Point, profileName, decoder *string, interval *int) types.Sensor {
	sensor := types.Sensor{SensorID: sensorID, DeviceID: deviceID, Name: name}
	if location.Valid {
		sensor.Location = &types.Location{Latitude: location.P.Y, Longitude: location.P.X}
	}
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
