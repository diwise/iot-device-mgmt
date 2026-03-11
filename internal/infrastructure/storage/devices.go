package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *Storage) CreateSensorProfile(ctx context.Context, p types.SensorProfile) error {
	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	args := pgx.NamedArgs{
		"sensor_profile_id": strings.ToLower(strings.TrimSpace(p.Decoder)),
		"name":              strings.TrimSpace(p.Name),
		"decoder":           strings.TrimSpace(p.Decoder),
		"interval":          p.Interval,
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO sensor_profiles (sensor_profile_id, name, decoder, interval)
		VALUES (@sensor_profile_id, @name, @decoder, @interval)
		ON CONFLICT DO NOTHING`, args)
	if err != nil {
		return err
	}

	for _, t := range p.Types {
		args["sensor_profile_type_id"] = strings.TrimSpace(t)
		_, err := tx.Exec(ctx, `
			INSERT INTO sensor_profiles_sensor_profile_types (sensor_profile_id, sensor_profile_type_id)
			VALUES (@sensor_profile_id, @sensor_profile_type_id)
			ON CONFLICT DO NOTHING`, args)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Storage) CreateSensorProfileType(ctx context.Context, t types.Lwm2mType) error {
	args := pgx.NamedArgs{
		"sensor_profile_type_id": strings.ToLower(strings.TrimSpace(t.Urn)),
		"name":                   strings.TrimSpace(t.Name),
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO sensor_profile_types (sensor_profile_type_id, name)
		VALUES (@sensor_profile_type_id, @name)
		ON CONFLICT DO NOTHING`, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) SetSensorProfile(ctx context.Context, deviceID string, dp types.SensorProfile) error {
	if dp.Decoder == "" {
		return fmt.Errorf("device profile contains no decoder")
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	args := pgx.NamedArgs{
		"sensor_profile": strings.ToLower(dp.Decoder),
		"device_id":      deviceID,
		"interval":       dp.Interval,
	}

	_, err = tx.Exec(ctx, `
		UPDATE devices SET
			interval=@interval,
			modified_on=NOW()
		WHERE device_id=@device_id AND deleted=FALSE`, args)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE sensors s
		SET sensor_profile=@sensor_profile,
			modified_on=NOW()
		FROM devices d
		WHERE d.device_id=@device_id
			AND d.deleted=FALSE
			AND d.sensor_id = s.sensor_id`, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) SetDeviceProfileTypes(ctx context.Context, deviceID string, types []types.Lwm2mType) error {
	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	args := pgx.NamedArgs{
		"device_id": deviceID,
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_sensor_profile_types WHERE device_id=@device_id;`, args)
	if err != nil {
		return err
	}

	for _, t := range types {
		if strings.TrimSpace(t.Urn) == "" {
			continue
		}

		args["sensor_profile_type_id"] = strings.TrimSpace(t.Urn)
		_, err = tx.Exec(ctx, `
			INSERT INTO device_sensor_profile_types (device_id, sensor_profile_type_id)
			VALUES (@device_id, @sensor_profile_type_id)
			ON CONFLICT DO NOTHING;`, args)
		if err != nil {
			log.Error("could not add type to device", "args", args, "err", err.Error())
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Storage) CreateOrUpdateDevice(ctx context.Context, d types.Device) error {
	log := logging.GetFromContext(ctx)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	sensorID := strings.TrimSpace(d.SensorID)
	args := pgx.NamedArgs{
		"device_id":   strings.TrimSpace(d.DeviceID),
		"active":      d.Active,
		"name":        strings.TrimSpace(d.Name),
		"description": d.Description,
		"environment": strings.TrimSpace(d.Environment),
		"source":      strings.TrimSpace(d.Source),
		"tenant":      strings.TrimSpace(d.Tenant),
		"lat":         d.Location.Latitude,
		"lon":         d.Location.Longitude,
		"interval":    d.Interval,
	}
	if sensorID == "" {
		args["sensor_id"] = nil
	} else {
		args["sensor_id"] = sensorID
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO devices (device_id,sensor_id,active,name,description,environment,source,tenant,location,interval)
		VALUES (@device_id,@sensor_id,@active,@name,@description,@environment,@source,@tenant,point(@lon,@lat),@interval)
		ON CONFLICT (device_id) DO UPDATE
			SET
				sensor_id = EXCLUDED.sensor_id,
				active = EXCLUDED.active,
				name = EXCLUDED.name,
				description = EXCLUDED.description,
				environment = EXCLUDED.environment,
				source = EXCLUDED.source,
				tenant = EXCLUDED.tenant,
				location = EXCLUDED.location,
				interval = EXCLUDED.interval,
				modified_on = NOW()
			WHERE devices.deleted = FALSE
		`, args)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_device_tags WHERE device_id=@device_id;`, args)
	if err != nil {
		return err
	}

	for _, t := range d.Tags {
		err = createTagTx(ctx, tx, t)
		if err != nil {
			return err
		}

		args["tag_name"] = strings.TrimSpace(t.Name)
		_, err = tx.Exec(ctx, `
			INSERT INTO device_device_tags (device_id, name)
			VALUES (@device_id, @tag_name)
			ON CONFLICT DO NOTHING;`, args)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_sensor_profile_types WHERE device_id=@device_id;`, args)
	if err != nil {
		return err
	}

	for _, t := range d.Lwm2mTypes {
		if strings.TrimSpace(t.Urn) == "" {
			continue
		}

		args["sensor_profile_type_id"] = strings.TrimSpace(t.Urn)
		_, err = tx.Exec(ctx, `
			INSERT INTO device_sensor_profile_types (device_id, sensor_profile_type_id)
			VALUES (@device_id, @sensor_profile_type_id)
			ON CONFLICT DO NOTHING;`, args)
		if err != nil {
			log.Error("could not add type to device", "args", args, "err", err.Error())
			return err
		}
	}

	for _, m := range d.Metadata {
		args["meta_key"] = strings.ToLower(strings.TrimSpace(m.Key))
		args["meta_value"] = strings.TrimSpace(m.Value)

		_, err = tx.Exec(ctx, `
			INSERT INTO device_metadata (device_id, key, vs)
			VALUES (@device_id, @meta_key, @meta_value)
			ON CONFLICT (device_id, key) DO UPDATE
				SET	vs = EXCLUDED.vs;`, args)
		if err != nil {
			log.Error("could not add metadata to device", "args", args, "err", err.Error())
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *Storage) UpdateDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error {
	if deviceID == "" {
		return ErrNoID
	}

	args := pgx.NamedArgs{
		"device_id": deviceID,
	}

	values := []string{}

	if active != nil {
		args["active"] = *active
		values = append(values, "active=@active")
	}

	if name != nil {
		args["name"] = *name
		values = append(values, "name=@name")
	}

	if description != nil {
		args["description"] = *description
		values = append(values, "description=@description")
	}

	if environment != nil {
		args["environment"] = *environment
		values = append(values, "environment=@environment")
	}

	if source != nil {
		args["source"] = *source
		values = append(values, "source=@source")
	}

	if tenant != nil {
		args["tenant"] = *tenant
		values = append(values, "tenant=@tenant")
	}

	if location != nil {
		args["lat"] = location.Latitude
		args["lon"] = location.Longitude
		values = append(values, "location=point(@lon,@lat)")
	}

	if interval != nil {
		args["interval"] = *interval
		values = append(values, "interval=@interval")
	}

	if len(values) == 0 {
		return nil
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	sql := "UPDATE devices SET " + strings.Join(values, ",") + " WHERE device_id=@device_id AND deleted=FALSE"

	_, err = tx.Exec(ctx, sql, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) AddTag(ctx context.Context, deviceID string, t types.Tag) error {
	if deviceID == "" {
		return ErrNoID
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	args := pgx.NamedArgs{
		"device_id": deviceID,
		"tag_name":  strings.TrimSpace(t.Name),
	}

	_, err = tx.Exec(ctx, `INSERT INTO device_device_tags (device_id, name) VALUES (@device_id, @tag_name) ON CONFLICT DO NOTHING;`, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) CreateTag(ctx context.Context, t types.Tag) error {
	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = createTagTx(ctx, tx, t)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func createTagTx(ctx context.Context, tx pgx.Tx, t types.Tag) error {
	args := pgx.NamedArgs{
		"name": strings.TrimSpace(t.Name),
	}
	_, err := tx.Exec(ctx, `INSERT INTO device_tags (name) VALUES (@name) ON CONFLICT DO NOTHING;`, args)
	return err
}

func (s *Storage) AddDeviceStatus(ctx context.Context, status types.StatusMessage) error {
	args := pgx.NamedArgs{
		"observed_at":   status.Timestamp.UTC(),
		"lookup_id":     status.DeviceID,
		"battery_level": status.BatteryLevel,
		"rssi":          status.RSSI,
		"snr":           status.LoRaSNR,
		"fq":            status.Frequency,
		"sf":            status.SpreadingFactor,
		"dr":            status.DR,
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var sensorID string
	err = tx.QueryRow(ctx, `
		SELECT sensor_id
		FROM devices
		WHERE deleted=FALSE
			AND sensor_id IS NOT NULL
			AND (device_id=@lookup_id OR sensor_id=@lookup_id)
		ORDER BY device_id ASC
		LIMIT 1`, args).Scan(&sensorID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrStatusDeviceNotFound
		}
		return err
	}
	args["sensor_id"] = sensorID

	_, err = tx.Exec(ctx, `
		INSERT INTO sensor_status (observed_at, sensor_id, battery_level, rssi, snr, fq, sf, dr)
		VALUES (@observed_at, @sensor_id, @battery_level, @rssi, @snr, @fq, @sf, @dr)
		ON CONFLICT DO NOTHING;`, args)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM sensor_status WHERE sensor_id=@sensor_id AND observed_at < NOW() - INTERVAL '3 weeks'`, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) GetDeviceStatus(ctx context.Context, deviceID string, query dmquery.Status) (types.Collection[types.SensorStatus], error) {
	log := logging.GetFromContext(ctx)

	condition := statusConditionFromQuery(deviceID, query)

	offsetLimitSql, offset, limit := OffsetLimit(condition, 0, 100)

	sql := fmt.Sprintf(`
		SELECT observed_at, battery_level, rssi, snr, fq, sf, dr, total_count
		FROM (
			SELECT ss.observed_at, ss.battery_level, ss.rssi, ss.snr, ss.fq, ss.sf, ss.dr, count(*) OVER () AS total_count
			FROM devices d
			JOIN sensor_status ss ON d.sensor_id = ss.sensor_id
			WHERE d.device_id=@device_id
			  AND d.tenant=ANY(@tenants)
			ORDER BY ss.observed_at DESC
			%s
		) AS statuses
		ORDER BY observed_at ASC;
		`, offsetLimitSql)

	args := NamedArgs(condition)
	args["device_id"] = deviceID

	now := time.Now()

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[types.SensorStatus]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, sql, args)
	if err != nil {
		log.Debug("failed to query device statuses", "sql", sql, "args", args, "err", err.Error())
		return types.Collection[types.SensorStatus]{}, err
	}
	defer rows.Close()

	log.Debug("GetDeviceStatus", slog.String("sql", sql), slog.Any("args", args), slog.Duration("duration", time.Duration(time.Since(now).Milliseconds())))

	statuses := []types.SensorStatus{}

	var count int64
	var observed_at time.Time
	var battery_level, rssi, snr, sf *float64
	var fq *int64
	var dr *int

	for rows.Next() {
		err := rows.Scan(&observed_at, &battery_level, &rssi, &snr, &fq, &sf, &dr, &count)
		if err != nil {
			return types.Collection[types.SensorStatus]{}, err
		}

		_rssi := rssi
		_snr := snr
		_fq := fq
		_sf := sf
		_dr := dr
		_observedAt := observed_at

		status := types.SensorStatus{
			RSSI:            _rssi,
			LoRaSNR:         _snr,
			Frequency:       _fq,
			SpreadingFactor: _sf,
			DR:              _dr,
			ObservedAt:      _observedAt.UTC(),
		}
		if battery_level != nil {
			status.BatteryLevel = int(*battery_level)
		}

		statuses = append(statuses, status)
	}

	if err := rows.Err(); err != nil {
		return types.Collection[types.SensorStatus]{}, err
	}

	return types.Collection[types.SensorStatus]{
		Data:       statuses,
		Count:      uint64(len(statuses)),
		TotalCount: uint64(count),
		Offset:     uint64(offset),
		Limit:      uint64(limit),
	}, nil
}

func (s *Storage) SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error {
	args := pgx.NamedArgs{
		"device_id":   deviceID,
		"observed_at": state.ObservedAt.UTC(),
		"online":      state.Online,
		"state":       state.State,
	}

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO device_state (device_id, observed_at, online, state)
		VALUES (@device_id, @observed_at, @online, @state)
		ON CONFLICT (device_id) DO UPDATE
			SET
				observed_at = EXCLUDED.observed_at,
				online = EXCLUDED.online,
				state = EXCLUDED.state,
				modified_on = NOW();
		`, args)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) GetDeviceBySensorID(ctx context.Context, sensorID string) (types.Device, bool, error) {
	if sensorID == "" {
		return types.Device{}, false, ErrNoID
	}

	args := pgx.NamedArgs{
		"sensor_id": sensorID,
	}

	log := logging.GetFromContext(ctx)

	var device_id, sensor_id, name, description, environment, source, tenant, device_profile string
	var active bool
	var location pgtype.Point
	var typesList [][]string

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Device{}, false, err
	}
	defer c.Release()

	row := c.QueryRow(ctx, `
		WITH types_list AS (
			SELECT ddpt.device_id, array_agg(ARRAY[dpt.sensor_profile_type_id, dpt.name]) AS types
			FROM device_sensor_profile_types ddpt
			JOIN sensor_profile_types dpt USING (sensor_profile_type_id)
			JOIN devices d USING (device_id)
			WHERE d.deleted = FALSE
			GROUP BY ddpt.device_id
		)

		SELECT d.device_id,d.sensor_id,active,name,description,environment,d.source,tenant,location,s.sensor_profile,types_list.types
		FROM devices d
		LEFT JOIN sensors s ON s.sensor_id = d.sensor_id
		LEFT JOIN types_list ON types_list.device_id = d.device_id
		WHERE d.sensor_id=@sensor_id AND d.deleted=FALSE`, args)

	err = row.Scan(&device_id, &sensor_id, &active, &name, &description, &environment, &source, &tenant, &location, &device_profile, &typesList)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Device{}, false, nil
		}
		log.Debug(fmt.Sprintf("query by sensorID %s did not return any data, reason: %v", sensorID, err))
		return types.Device{}, false, err
	}

	d := types.Device{
		Active:      active,
		SensorID:    sensor_id,
		DeviceID:    device_id,
		Tenant:      tenant,
		Name:        name,
		Description: description,
		Environment: environment,
		Source:      source,
		Location: types.Location{
			Latitude:  location.P.Y,
			Longitude: location.P.X,
		},
		SensorProfile: types.SensorProfile{
			Decoder: device_profile,
			Name:    device_profile,
		},
	}

	if len(typesList) > 0 {
		for _, t := range typesList {
			d.Lwm2mTypes = append(d.Lwm2mTypes, types.Lwm2mType{
				Urn:  t[0],
				Name: t[1],
			})
		}
	}

	return d, true, nil
}

func (s *Storage) Query(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
	log := logging.GetFromContext(ctx)

	condition := deviceConditionFromQuery(query.Filters)
	offsetLimit, offset, limit := OffsetLimit(condition, 0, 10)

	if condition.Export {
		offsetLimit = ""
		offset = 0
	}

	sql := fmt.Sprintf(`
		WITH latest_status AS (
			SELECT DISTINCT ON (sensor_id)
				sensor_id, battery_level, rssi, snr, fq, sf, dr, observed_at
			FROM sensor_status
			ORDER BY sensor_id, observed_at DESC
		),

		tag_list AS (
			SELECT ddt.device_id, array_agg(dt.name) AS tags
			FROM device_device_tags ddt
			JOIN device_tags dt USING (name)
			GROUP BY ddt.device_id
		),

		metadata_list AS (
			SELECT dm.device_id, array_agg(ARRAY[dm.key, dm.vs]) AS meta
			FROM device_metadata dm
			GROUP BY dm.device_id
		),

		types_list AS (
			SELECT ddpt.device_id, array_agg(ARRAY[dpt.sensor_profile_type_id, dpt.name]) AS types
			FROM device_sensor_profile_types ddpt
			JOIN sensor_profile_types dpt USING (sensor_profile_type_id)
			JOIN devices d USING (device_id)
			WHERE d.deleted = FALSE
			GROUP BY ddpt.device_id
		),

		alarms_list AS (
			SELECT a.device_id, array_agg(a.type) AS alarms
			FROM device_alarms a
			GROUP BY a.device_id
		)

		SELECT
			d.device_id,
			d.sensor_id,
			d.active,
			d.location,
			d.name           AS device_name,
			d.description    AS device_description,
			d.environment,
			d.source,
			d.tenant,

			sp.sensor_profile_id,
			sp.name          AS profile_name,
			sp.decoder,
			sp.description   AS profile_description,
			sp.interval      AS profile_interval,
			d.interval	     AS device_interval,

			dst.online      AS state_online,
			dst.state       AS state_value,
			dst.observed_at AS state_observed_at,

			ls.battery_level,
			ls.rssi,
			ls.snr,
			ls.fq,
			ls.sf,
			ls.dr,
			ls.observed_at  AS status_observed_at,

			tl.tags,
			ml.meta,
			types_list.types,

			alarms_list.alarms,

			count(*) OVER () AS count

		FROM devices d
		LEFT JOIN sensors s ON s.sensor_id = d.sensor_id
		LEFT JOIN sensor_profiles sp ON sp.sensor_profile_id = s.sensor_profile
		LEFT JOIN device_state dst ON dst.device_id = d.device_id
		LEFT JOIN latest_status ls ON ls.sensor_id = d.sensor_id
		LEFT JOIN tag_list tl ON tl.device_id = d.device_id
		LEFT JOIN types_list ON types_list.device_id = d.device_id
		LEFT JOIN alarms_list ON alarms_list.device_id = d.device_id
		LEFT JOIN metadata_list ml ON ml.device_id = d.device_id
		%s
		%s
		%s;`, Where(condition), OrderByWithFallback(condition, "ORDER BY active DESC, state_observed_at DESC NULLS LAST, device_id ASC"), offsetLimit)

	args := NamedArgs(condition)

	now := time.Now()

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, sql, args)
	if err != nil {
		log.Debug("failed to query database", "sql", sql, "args", args, "err", err.Error())
		return types.Collection[types.Device]{}, err
	}
	defer rows.Close()

	log.Debug("Query", slog.String("sql", sql), slog.Any("args", args), slog.Duration("duration", time.Duration(time.Since(now).Milliseconds())))

	var devices []types.Device
	var count uint64
	for rows.Next() {
		var location pgtype.Point
		var profileName, profileDescription, deviceName, deviceDescription, environment, source, tenant *string
		var deviceID, sensorID, profileID *string
		var active, online *bool
		var rssi, snr, sf, batteryLevel *float64
		var statusObservedAt, stateObservedAt *time.Time
		var tagList, alarmsList []string
		var typesList, metadataList [][]string
		var decoder *string
		var interval, deviceInterval, stateValue, dr *int
		var fq *int64

		err = rows.Scan(
			&deviceID,
			&sensorID,
			&active,
			&location,
			&deviceName,
			&deviceDescription,
			&environment,
			&source,
			&tenant,
			&profileID,
			&profileName,
			&decoder,
			&profileDescription,
			&interval,
			&deviceInterval,
			&online,
			&stateValue,
			&stateObservedAt,
			&batteryLevel,
			&rssi,
			&snr,
			&fq,
			&sf,
			&dr,
			&statusObservedAt,
			&tagList,
			&metadataList,
			&typesList,
			&alarmsList,
			&count,
		)
		if err != nil {
			log.Debug("could not scan row", "err", err.Error())
			return types.Collection[types.Device]{}, err
		}

		device := types.Device{
			DeviceID: *deviceID,
			Active:   *active,
			Tenant:   *tenant,
		}

		if sensorID != nil {
			device.SensorID = *sensorID
		}
		if deviceName != nil {
			device.Name = *deviceName
		}
		if deviceDescription != nil {
			device.Description = *deviceDescription
		}
		if environment != nil {
			device.Environment = *environment
		}
		if source != nil {
			device.Source = *source
		}
		device.Location = types.Location{
			Latitude:  location.P.Y,
			Longitude: location.P.X,
		}

		if profileID != nil {
			device.SensorProfile = types.SensorProfile{
				Name:     *profileID,
				Decoder:  *decoder,
				Interval: *interval,
			}
			if deviceInterval != nil && *deviceInterval > 0 {
				device.SensorProfile.Interval = *deviceInterval
			}
		}
		if stateObservedAt != nil {
			device.DeviceState = types.DeviceState{
				Online:     *online,
				State:      *stateValue,
				ObservedAt: stateObservedAt.UTC(),
			}
		}
		if statusObservedAt != nil {
			device.SensorStatus = types.SensorStatus{
				RSSI:            rssi,
				LoRaSNR:         snr,
				Frequency:       fq,
				SpreadingFactor: sf,
				DR:              dr,
				ObservedAt:      statusObservedAt.UTC(),
			}
			if batteryLevel != nil {
				bat := *batteryLevel
				device.SensorStatus.BatteryLevel = int(bat)
			}
		}
		if len(tagList) > 0 {
			device.Tags = make([]types.Tag, 0)
			for _, t := range tagList {
				device.Tags = append(device.Tags, types.Tag{
					Name: t,
				})
			}
		}
		if len(metadataList) > 0 {
			device.Metadata = make([]types.Metadata, 0)
			for _, m := range metadataList {
				if len(m) == 2 && m[0] != "" {
					device.Metadata = append(device.Metadata, types.Metadata{
						Key:   m[0],
						Value: m[1],
					})
				}
			}
		}
		if len(typesList) > 0 {
			device.Lwm2mTypes = make([]types.Lwm2mType, 0)
			for _, t := range typesList {
				device.Lwm2mTypes = append(device.Lwm2mTypes, types.Lwm2mType{
					Urn:  t[0],
					Name: t[1],
				})
			}
		}
		if len(alarmsList) > 0 {
			device.Alarms = append(device.Alarms, alarmsList...)
		}

		devices = append(devices, device)
	}

	if err := rows.Err(); err != nil {
		return types.Collection[types.Device]{}, err
	}

	if condition.Export {
		limit = len(devices)
	}

	return types.Collection[types.Device]{
		Data:       devices,
		Count:      uint64(len(devices)),
		TotalCount: count,
		Offset:     uint64(offset),
		Limit:      uint64(limit),
	}, nil
}

func (s *Storage) GetDeviceMeasurements(ctx context.Context, deviceID string, query dmquery.Measurements) (types.Collection[types.Measurement], error) {
	condition := measurementConditionFromQuery(deviceID, query)

	args := NamedArgs(condition)
	offsetLimit, offset, limit := OffsetLimit(condition)

	if offsetLimit == "" {
		offsetLimit = "OFFSET 0 LIMIT 10 "
	}

	sql := fmt.Sprintf(`
		SELECT d."time",d.id,d.urn,d.n,d.v,d.vs,d.vb,d.unit, count(*) OVER () AS count
		FROM events_measurements d
		-- LEFT JOIN sensor_profile_types spt ON d.urn=spt.sensor_profile_type_id
		%s
		ORDER BY d."time" DESC
		%s`, Where(condition), offsetLimit)

	c, err := s.conn.Acquire(ctx)
	if err != nil {
		return types.Collection[types.Measurement]{}, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, sql, args)
	if err != nil {
		return types.Collection[types.Measurement]{}, err
	}
	defer rows.Close()

	measurements := []types.Measurement{}
	var totalCount int64
	var ts time.Time
	var id, urn, n string
	var v *float64
	var vs, unit *string
	var vb *bool

	for rows.Next() {
		err := rows.Scan(&ts, &id, &urn, &n, &v, &vs, &vb, &unit, &totalCount)
		if err != nil {
			return types.Collection[types.Measurement]{}, err
		}

		m := types.Measurement{
			ID: strings.Replace(id, deviceID, "", 1),
			//Urn:       urn,
			Unit:      unit,
			Timestamp: ts.UTC(),
		}
		/*
			if name != nil {
				n := strings.ToLower(*name)
				m.Name = &n
			}
		*/
		if v != nil {
			m.Value = *v
		} else if vs != nil {
			m.Value = *vs
		} else if vb != nil {
			m.Value = *vb
		}

		measurements = append(measurements, m)
	}

	if err := rows.Err(); err != nil {
		return types.Collection[types.Measurement]{}, err
	}

	return types.Collection[types.Measurement]{
		Data:       measurements,
		Count:      uint64(len(measurements)),
		TotalCount: uint64(totalCount),
		Offset:     uint64(offset),
		Limit:      uint64(limit),
	}, nil
}
