package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/jackc/pgx/v5"
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
	SetDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source *string, location *types.Location) error
}

type Storage struct {
	pool *pgxpool.Pool
}

func NewWithPool(pool *pgxpool.Pool) *Storage {
	return &Storage{pool: pool}
}

func New(ctx context.Context, config Config) (*Storage, error) {
	pool, err := NewPool(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Storage{pool: pool}, nil
}

func (s *Storage) Initialize(ctx context.Context) error {
	return s.CreateTables(ctx)
}

func (s *Storage) CreateTables(ctx context.Context) error {
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
			device_id		TEXT NOT NULL,
			type			TEXT NOT NULL,
			description		TEXT NULL,
			created_on  	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			
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

func (s *Storage) Close() {
	s.pool.Close()
}

func (s *Storage) CreateDeviceProfileType(ctx context.Context, t types.Lwm2mType) error {
	args := pgx.NamedArgs{
		"device_profile_type_id": strings.TrimSpace(t.Urn),
		"name":                   strings.TrimSpace(t.Name),
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO device_profiles_types (device_profile_type_id, name) VALUES (@device_profile_type_id, @name) ON CONFLICT DO NOTHING`, args)
	return err
}

func (s *Storage) CreateDeviceProfile(ctx context.Context, p types.DeviceProfile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"device_profile_id": strings.TrimSpace(p.Decoder),
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

func (s *Storage) CreateOrUpdateDevice(ctx context.Context, d types.Device) error {
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
		"device_profile": strings.TrimSpace(d.DeviceProfile.Name),
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

func (s *Storage) AddTag(ctx context.Context, deviceID string, t types.Tag) error {
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

func (s *Storage) CreateTag(ctx context.Context, t types.Tag) error {
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

func (s *Storage) AddDeviceStatus(ctx context.Context, status types.StatusMessage) error {
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

	_, err = tx.Exec(ctx, `DELETE FROM device_status WHERE device_id=@device_id AND time < NOW() - INTERVAL '3 weeks'`, args)
	if err != nil {
		tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}

func (s *Storage) SetDeviceState(ctx context.Context, deviceID string, state types.DeviceState) error {
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

func (s *Storage) SetDeviceProfile(ctx context.Context, deviceID string, dp types.DeviceProfile) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"device_profile": dp.Name,
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

func (s *Storage) SetDevice(ctx context.Context, deviceID string, active *bool, name, description, environment, source *string, location *types.Location) error {
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

func (s *Storage) Query(ctx context.Context) (any, error) {
	args := pgx.NamedArgs{}
	sql := `
WITH latest_status AS (
  SELECT DISTINCT ON (device_id)
    device_id, battery_level, rssi, snr, fq, sf, dr, observed_at
  FROM device_status
  ORDER BY device_id, observed_at DESC
),

tag_list AS (
  SELECT
    ddt.device_id,
    array_agg(dt.name) AS tags
  FROM device_device_tags ddt
  JOIN device_tags dt USING (name)
  GROUP BY ddt.device_id
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
  d.interval	   AS device_interval,

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

  tl.tags
FROM devices d
LEFT JOIN device_profiles dp
  ON dp.device_profile_id = d.device_profile
LEFT JOIN device_state dst
  ON dst.device_id = d.device_id
LEFT JOIN latest_status ls
  ON ls.device_id = d.device_id
LEFT JOIN tag_list tl
  ON tl.device_id = d.device_id;`

	_, err := s.pool.Query(ctx, sql, args)
	if err != nil {
		return nil, err
	}

	return nil, nil

}
