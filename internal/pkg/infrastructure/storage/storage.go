package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	host     string
	user     string
	password string
	port     string
	dbname   string
	sslmode  string
}

func (c Config) ConnStr() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", c.user, c.password, c.host, c.port, c.dbname, c.sslmode)
}

func NewConfig(host, user, password, port, dbname, sslmode string) Config {
	return Config{
		host:     host,
		user:     user,
		password: password,
		port:     port,
		dbname:   dbname,
		sslmode:  sslmode,
	}
}

func NewPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	p, err := pgxpool.New(ctx, config.ConnStr())
	if err != nil {
		return nil, err
	}

	err = p.Ping(ctx)
	if err != nil {
		return nil, err
	}

	return p, nil
}

var (
	ErrNoRows        = errors.New("no rows in result set")
	ErrTooManyRows   = errors.New("too many rows in result set")
	ErrQueryRow      = errors.New("could not execute query")
	ErrStoreFailed   = errors.New("could not store data")
	ErrNoID          = errors.New("data contains no id")
	ErrMissingTenant = errors.New("missing tenant information")
	ErrAlreadyExist  = errors.New("device already exists")
	ErrDeleted       = errors.New("deleted")
)

//go:generate moq -rm -out store_mock.go . Store
type Store interface {
	Initialize(ctx context.Context) error
	Close()

	CreateDeviceProfile(ctx context.Context, p types.DeviceProfile) error
	CreateDeviceProfileType(ctx context.Context, t types.Lwm2mType) error
	CreateOrUpdateDevice(ctx context.Context, d types.Device) error
	CreateTag(ctx context.Context, t types.Tag) error

	AddTag(ctx context.Context, deviceID string, t types.Tag) error
	AddDeviceStatus(ctx context.Context, status types.StatusMessage) error
	SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error
	SetDeviceProfile(ctx context.Context, deviceID string, dp types.DeviceProfile) error
	SetDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location) error
	SetDeviceProfileTypes(ctx context.Context, deviceID string, types []types.Lwm2mType) error

	Query(ctx context.Context, conditions ...ConditionFunc) (types.Collection[types.Device], error)

	GetTenants(ctx context.Context) (types.Collection[string], error)

	AddAlarm(ctx context.Context, deviceID string, a types.Alarm) error
	GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error)
	RemoveAlarm(ctx context.Context, deviceID string, alarmType string) error
}

type storageImpl struct {
	pool *pgxpool.Pool
}

func NewWithPool(pool *pgxpool.Pool) Store {
	return &storageImpl{pool: pool}
}

func New(ctx context.Context, config Config) (Store, error) {
	pool, err := NewPool(ctx, config)
	if err != nil {
		return nil, err
	}

	return &storageImpl{pool: pool}, nil
}

func (s *storageImpl) Initialize(ctx context.Context) error {
	return createTables(ctx, s)
}

func createTables(ctx context.Context, s *storageImpl) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS device_profiles (
			device_profile_id	TEXT NOT NULL,
			name 				TEXT NULL,
			decoder 			TEXT NOT NULL,
			description			TEXT NULL,
			interval 			NUMERIC NOT NULL DEFAULT 3600,
			created_on  		timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_profiles PRIMARY KEY (device_profile_id)
		);

		CREATE TABLE IF NOT EXISTS device_profiles_types (
			device_profile_type_id	TEXT NOT NULL,
			name 					TEXT NULL,
			created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_profiles_types PRIMARY KEY (device_profile_type_id)
		);

		CREATE TABLE IF NOT EXISTS devices (
			device_id	TEXT 	NOT NULL,
			sensor_id	TEXT 	NULL,

			active		BOOLEAN	NOT NULL DEFAULT FALSE,

			name        TEXT 	NULL,
			description TEXT 	NULL,
			environment TEXT 	NULL,
			source      TEXT 	NULL,
			tenant		TEXT 	NOT NULL,
			location 	POINT 	NULL,

			device_profile 	TEXT NULL,
			interval 		NUMERIC NOT NULL DEFAULT 0,

			created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_on timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted     BOOLEAN DEFAULT FALSE,
			deleted_on  timestamp with time zone NULL,

			CONSTRAINT pk_devices PRIMARY KEY (device_id),
			CONSTRAINT fk_device_profiles FOREIGN KEY (device_profile) REFERENCES device_profiles (device_profile_id) ON DELETE SET NULL
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uq_devices_sensor_not_deleted ON devices(device_id, sensor_id) WHERE deleted = FALSE;

		CREATE TABLE IF NOT EXISTS device_tags (
			name  		TEXT NOT NULL,
			created_on	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_tags PRIMARY KEY (name)
		);

		CREATE TABLE IF NOT EXISTS device_status (
			observed_at		timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			device_id		TEXT 	NOT NULL,
			battery_level 	NUMERIC NULL,
			rssi 			NUMERIC NULL,
			snr 			NUMERIC NULL,
			fq 				NUMERIC NULL,
			sf 				NUMERIC NULL,
			dr 				NUMERIC NULL,
			created_on  	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_status PRIMARY KEY (observed_at, device_id),
			CONSTRAINT fk_device_device_status FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS device_state (
			device_id	TEXT NOT NULL,
			online 		BOOLEAN NOT NULL DEFAULT FALSE,
			state 		NUMERIC NOT NULL DEFAULT -1,
			observed_at	timestamp with time zone NULL,
			created_on 	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			modified_on timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_state PRIMARY KEY (device_id),
			CONSTRAINT fk_device_device_state FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS device_alarms (
			device_id	TEXT NOT NULL,
			type		TEXT NOT NULL,
			description	TEXT NULL,
			severity	NUMERIC NOT NULL DEFAULT 0,
			observed_at	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_alarms PRIMARY KEY (device_id, type),
			CONSTRAINT fk_device_alarms FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS device_device_tags (
			device_id 	TEXT NOT NULL,
			name  		TEXT NOT NULL,
			created_on	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_device_tags PRIMARY KEY (device_id, name),
			CONSTRAINT fk_device_device_tags_device FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE,
			CONSTRAINT fk_device_device_tags_tags FOREIGN KEY (name) REFERENCES device_tags (name) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS device_profiles_device_profiles_types (
			device_profile_id 		TEXT NOT NULL,
			device_profile_type_id	TEXT NOT NULL,
			created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_profiles_device_profiles_types PRIMARY KEY (device_profile_id, device_profile_type_id),
			CONSTRAINT fk_device_profiles_device_profiles_types FOREIGN KEY (device_profile_id) REFERENCES device_profiles (device_profile_id) ON DELETE CASCADE,
			CONSTRAINT fk_device_profiles_device_profiles_types_type FOREIGN KEY (device_profile_type_id) REFERENCES device_profiles_types (device_profile_type_id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS device_device_profile_types (
			device_id 				TEXT NOT NULL,
			device_profile_type_id	TEXT NOT NULL,
			created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,

			CONSTRAINT pk_device_device_profile_types PRIMARY KEY (device_id, device_profile_type_id),
			CONSTRAINT fk_device_device_profile_types FOREIGN KEY (device_id) REFERENCES devices (device_id) ON DELETE CASCADE,
			CONSTRAINT fk_device_device_profile_types_type FOREIGN KEY (device_profile_type_id) REFERENCES device_profiles_types (device_profile_type_id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_device_state_device_id ON device_state(device_id);
		CREATE INDEX IF NOT EXISTS idx_device_device_tags_name ON device_device_tags(name);
		CREATE INDEX IF NOT EXISTS idx_device_device_profile_types_type ON device_device_profile_types(device_profile_type_id);
	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *storageImpl) Close() {
	s.pool.Close()
}

func (s *storageImpl) CreateDeviceProfileType(ctx context.Context, t types.Lwm2mType) error {
	args := pgx.NamedArgs{
		"device_profile_type_id": strings.ToLower(strings.TrimSpace(t.Urn)),
		"name":                   strings.TrimSpace(t.Name),
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO device_profiles_types (device_profile_type_id, name) VALUES (@device_profile_type_id, @name) ON CONFLICT DO NOTHING`, args)
	return err
}

func (s *storageImpl) CreateDeviceProfile(ctx context.Context, p types.DeviceProfile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"device_profile_id": strings.ToLower(strings.TrimSpace(p.Decoder)),
		"name":              strings.TrimSpace(p.Name),
		"decoder":           strings.TrimSpace(p.Decoder),
		"interval":          p.Interval,
	}

	_, err = tx.Exec(ctx, `INSERT INTO device_profiles (device_profile_id, name, decoder, interval) VALUES (@device_profile_id, @name, @decoder, @interval) ON CONFLICT DO NOTHING`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	for _, t := range p.Types {
		args["device_profile_type_id"] = strings.TrimSpace(t)
		_, err := tx.Exec(ctx, `INSERT INTO device_profiles_device_profiles_types (device_profile_id, device_profile_type_id) VALUES (@device_profile_id, @device_profile_type_id) ON CONFLICT DO NOTHING`, args)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) CreateOrUpdateDevice(ctx context.Context, d types.Device) error {
	log := logging.GetFromContext(ctx)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"device_id":      strings.TrimSpace(d.DeviceID),
		"sensor_id":      strings.TrimSpace(d.SensorID),
		"active":         d.Active,
		"name":           strings.TrimSpace(d.Name),
		"description":    d.Description,
		"environment":    strings.TrimSpace(d.Environment),
		"source":         d.Source,
		"tenant":         strings.TrimSpace(d.Tenant),
		"lat":            d.Location.Latitude,
		"lon":            d.Location.Longitude,
		"device_profile": strings.ToLower(strings.TrimSpace(d.DeviceProfile.Decoder)),
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO devices (device_id,sensor_id,active,name,description,environment,source,tenant,location,device_profile)
		VALUES (@device_id,@sensor_id,@active,@name,@description,@environment,@source,@tenant,point(@lon,@lat),@device_profile)
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
				device_profile = EXCLUDED.device_profile,
				modified_on = NOW()
			WHERE devices.deleted = FALSE
		`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_device_tags WHERE device_id=@device_id;`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	for _, t := range d.Tags {
		err = createTagTx(ctx, tx, t)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}

		args["tag_name"] = strings.TrimSpace(t.Name)
		_, err = tx.Exec(ctx, `INSERT INTO device_device_tags (device_id, name) VALUES (@device_id, @tag_name) ON CONFLICT DO NOTHING;`, args)
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_device_profile_types WHERE device_id=@device_id;`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	for _, t := range d.Lwm2mTypes {
		if strings.TrimSpace(t.Urn) == "" {
			continue
		}

		args["device_profile_type_id"] = strings.TrimSpace(t.Urn)
		_, err = tx.Exec(ctx, `INSERT INTO device_device_profile_types (device_id, device_profile_type_id) VALUES (@device_id, @device_profile_type_id) ON CONFLICT DO NOTHING;`, args)
		if err != nil {
			log.Error("could not add type to device", "args", args, "err", err.Error())
			tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) AddTag(ctx context.Context, deviceID string, t types.Tag) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"device_id": deviceID,
		"tag_name":  strings.TrimSpace(t.Name),
	}

	_, err = tx.Exec(ctx, `INSERT INTO device_device_tags (device_id, name) VALUES (@device_id, @tag_name) ON CONFLICT DO NOTHING;`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) CreateTag(ctx context.Context, t types.Tag) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	err = createTagTx(ctx, tx, t)
	if err != nil {
		tx.Rollback(ctx)
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

func (s *storageImpl) AddDeviceStatus(ctx context.Context, status types.StatusMessage) error {
	args := pgx.NamedArgs{
		"observed_at":   status.Timestamp,
		"device_id":     status.DeviceID,
		"battery_level": status.BatteryLevel,
		"rssi":          status.RSSI,
		"snr":           status.LoRaSNR,
		"fq":            status.Frequency,
		"sf":            status.SpreadingFactor,
		"dr":            status.DR,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO device_status (observed_at, device_id, battery_level, rssi, snr, fq, sf, dr)
		VALUES (@observed_at, @device_id, @battery_level, @rssi, @snr, @fq, @sf, @dr);`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_status WHERE device_id=@device_id AND observed_at < NOW() - INTERVAL '3 weeks'`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error {
	args := pgx.NamedArgs{
		"device_id":   deviceID,
		"observed_at": state.ObservedAt,
		"online":      state.Online,
		"state":       state.State,
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

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
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) SetDeviceProfile(ctx context.Context, deviceID string, dp types.DeviceProfile) error {
	if dp.Decoder == "" {
		return fmt.Errorf("device profile contains no decoder")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"device_profile": strings.ToLower(dp.Decoder),
		"device_id":      deviceID,
		"interval":       dp.Interval,
	}

	_, err = tx.Exec(ctx, `
		UPDATE devices SET
			device_profile=@device_profile,
			interval=@interval,
			modified_on=NOW()
		WHERE device_id=@device_id AND deleted=FALSE`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) SetDeviceProfileTypes(ctx context.Context, deviceID string, types []types.Lwm2mType) error {
	log := logging.GetFromContext(ctx)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"device_id": deviceID,
	}

	_, err = tx.Exec(ctx, `DELETE FROM device_device_profile_types WHERE device_id=@device_id;`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	for _, t := range types {
		if strings.TrimSpace(t.Urn) == "" {
			continue
		}

		args["device_profile_type_id"] = strings.TrimSpace(t.Urn)
		_, err = tx.Exec(ctx, `INSERT INTO device_device_profile_types (device_id, device_profile_type_id) VALUES (@device_id, @device_profile_type_id) ON CONFLICT DO NOTHING;`, args)
		if err != nil {
			log.Error("could not add type to device", "args", args, "err", err.Error())
			tx.Rollback(ctx)
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) SetDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location) error {
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
		args["tenant"] = *source
		values = append(values, "tenant=@tenant")
	}

	if location != nil {
		args["lat"] = location.Latitude
		args["lon"] = location.Longitude
		values = append(values, "location=point(@lon,@lat)")
	}

	if len(values) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	sql := "UPDATE devices SET " + strings.Join(values, ",") + " WHERE device_id=@device_id AND deleted=FALSE"

	_, err = tx.Exec(ctx, sql, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *storageImpl) Query(ctx context.Context, conditions ...ConditionFunc) (types.Collection[types.Device], error) {
	condition := &Condition{}
	for _, c := range conditions {
		c(condition)
	}

	sql := fmt.Sprintf(`
		WITH latest_status AS (
			SELECT DISTINCT ON (device_id)
				device_id, battery_level, rssi, snr, fq, sf, dr, observed_at
			FROM device_status
			ORDER BY device_id, observed_at DESC
		),

		tag_list AS (
			SELECT ddt.device_id, array_agg(dt.name) AS tags
			FROM device_device_tags ddt
			JOIN device_tags dt USING (name)
			GROUP BY ddt.device_id
		),

		types_list AS (
			SELECT ddpt.device_id, array_agg(ARRAY[dpt.device_profile_type_id, dpt.name]) AS types
			FROM device_device_profile_types ddpt
			JOIN device_profiles_types dpt USING (device_profile_type_id)
			JOIN devices d USING (device_id)
			WHERE d.deleted = FALSE
			GROUP BY ddpt.device_id
		),

		alarms_list AS (
			SELECT a.device_id, array_agg(a.type) AS alarms
			FROM device_alarms
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

			dp.device_profile_id,
			dp.name          AS profile_name,
			dp.decoder,
			dp.description   AS profile_description,
			dp.interval      AS profile_interval,
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
			types_list.types,

			alarms_list.alarms,

			count(*) OVER () AS count

		FROM devices d
		LEFT JOIN device_profiles dp ON dp.device_profile_id = d.device_profile
		LEFT JOIN device_state dst ON dst.device_id = d.device_id
		LEFT JOIN latest_status ls ON ls.device_id = d.device_id
		LEFT JOIN tag_list tl ON tl.device_id = d.device_id
		LEFT JOIN types_list ON types_list.device_id = d.device_id
		LEFT JOIN alarms_list ON alarms_list.device_id = d.device_id
		%s
		%s;`, condition.Where(), condition.OffsetLimit())

	rows, err := s.pool.Query(ctx, sql, condition.NamedArgs())
	if err != nil {
		return types.Collection[types.Device]{}, err
	}
	defer rows.Close()

	var devices []types.Device
	var count uint64
	for rows.Next() {
		var location pgtype.Point
		var profileName, profileDescription, deviceName, deviceDescription, environment, source, tenant *string
		var deviceID, sensorID, profileID *string
		var active, online *bool
		var rssi, snr, sf *float64
		var statusObservedAt, stateObservedAt *time.Time
		var tagList, alarmsList []string
		var typesList [][]string
		var decoder *string
		var interval, deviceInterval, stateValue, batteryLevel, dr *int
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
			&typesList,
			&alarmsList,
			&count,
		)
		if err != nil {
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
		if profileID != nil {
			device.DeviceProfile = types.DeviceProfile{
				Name:     *profileID,
				Decoder:  *decoder,
				Interval: *interval,
			}
			if deviceInterval != nil && *deviceInterval > 0 {
				device.DeviceProfile.Interval = *deviceInterval
			}
		}
		if stateObservedAt != nil {
			device.DeviceState = types.DeviceState{
				Online:     *online,
				State:      *stateValue,
				ObservedAt: *stateObservedAt,
			}
		}
		if statusObservedAt != nil {
			device.DeviceStatus = types.DeviceStatus{
				RSSI:            rssi,
				LoRaSNR:         snr,
				Frequency:       fq,
				SpreadingFactor: sf,
				DR:              dr,
				ObservedAt:      *statusObservedAt,
			}
			if batteryLevel != nil {
				device.DeviceStatus.BatteryLevel = *batteryLevel
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

	return types.Collection[types.Device]{
		Data:       devices,
		Count:      uint64(len(devices)),
		TotalCount: count,
		Offset:     0,
		Limit:      uint64(len(devices)),
	}, nil
}

func (s *storageImpl) GetTenants(ctx context.Context) (types.Collection[string], error) {
	rows, err := s.pool.Query(ctx, "SELECT DISTINCT tenant FROM devices ORDER BY tenant ASC", nil)
	if err != nil {
		return types.Collection[string]{}, err
	}
	defer rows.Close()

	tenants := []string{}

	for rows.Next() {
		var tenant string

		err := rows.Scan(&tenant)
		if err != nil {
			return types.Collection[string]{}, err
		}

		if tenant != "" {
			tenants = append(tenants, tenant)
		}
	}

	return types.Collection[string]{
		Data:       tenants,
		Count:      uint64(len(tenants)),
		Offset:     0,
		Limit:      uint64(len(tenants)),
		TotalCount: uint64(len(tenants)),
	}, nil
}

func (s *storageImpl) AddAlarm(ctx context.Context, deviceID string, a types.Alarm) error {
	args := pgx.NamedArgs{
		"device_id":   deviceID,
		"type":        a.AlarmType,
		"description": a.Description,
		"observed_at": a.ObservedAt,
		"severity":    a.Severity,
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO device_alarms (device_id, type, description, observed_at, severity) 
		VALUES (@device_id, @type, @description, @observed_at, @severity) 
		ON CONFLICT (device_id, type) DO UPDATE
			SET
				description=EXCLUDED.description,
				observed_at=EXCLUDED.observed_at,
				severity=EXCLUDED.severity
				`, args)

	return err
}

func (s *storageImpl) GetStaleDevices(ctx context.Context) (types.Collection[types.Device], error) {
	sql := `
		WITH last_status AS (
			SELECT device_id, MAX(observed_at) AS last_observed
			FROM device_status
			GROUP BY device_id
		)
		SELECT
			d.device_id,
			d.sensor_id,
			d.active,
			d.tenant,
			d.device_profile,
			d.interval      AS device_interval,
			dp.interval     AS profile_interval,
			ls.last_observed,
			CASE WHEN d.interval = 0 THEN dp.interval ELSE d.interval END AS effective_interval_seconds
		FROM devices d
			LEFT JOIN device_profiles dp ON dp.device_profile_id = d.device_profile
			LEFT JOIN last_status ls ON ls.device_id = d.device_id
		WHERE ls.last_observed IS NULL OR ls.last_observed < NOW() - (COALESCE(NULLIF(d.interval, 0), dp.interval) * INTERVAL '1 second');`

	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}
	defer rows.Close()

	devices := []types.Device{}

	for rows.Next() {
		var deviceID, tenant, profile string
		var device_interval, profile_interval, effective_interval int
		var sensorID *string
		var active bool
		var lastObserved *time.Time

		err := rows.Scan(&deviceID, &sensorID, &active, &tenant, &profile, &device_interval, &profile_interval, &lastObserved, &effective_interval)
		if err != nil {
			return types.Collection[types.Device]{}, err
		}

		devices = append(devices, types.Device{
			Active:   active,
			SensorID: *sensorID,
			DeviceID: deviceID,
			Tenant:   tenant,
		})
	}

	return types.Collection[types.Device]{
		Data:       devices,
		Count:      uint64(len(devices)),
		TotalCount: uint64(len(devices)),
		Offset:     0,
		Limit:      uint64(len(devices)),
	}, nil
}

func (s *storageImpl) RemoveAlarm(ctx context.Context, deviceID string, alarmType string) error {
	args := pgx.NamedArgs{
		"device_id":  deviceID,
		"alarm_type": alarmType,
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM device_alarms WHERE device_id=@device_id AND type=@alarm_type`, args)

	return err
}
