package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/jackc/pgx/pgtype"
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

func LoadConfiguration(ctx context.Context) Config {
	return Config{
		host:     env.GetVariableOrDefault(ctx, "POSTGRES_HOST", ""),
		user:     env.GetVariableOrDefault(ctx, "POSTGRES_USER", ""),
		password: env.GetVariableOrDefault(ctx, "POSTGRES_PASSWORD", ""),
		port:     env.GetVariableOrDefault(ctx, "POSTGRES_PORT", "5432"),
		dbname:   env.GetVariableOrDefault(ctx, "POSTGRES_DBNAME", "diwise"),
		sslmode:  env.GetVariableOrDefault(ctx, "POSTGRES_SSLMODE", "disable"),
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
)

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

func (s *Storage) CreateTables(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS devices (
			device_id	TEXT 	NOT NULL,			
			sensor_id	TEXT 	NOT NULL,	
			active		BOOLEAN	NOT NULL DEFAULT FALSE,					
			data 		JSONB	NOT NULL,				
			profile 	JSONB	NOT NULL,
			state 		JSONB	NULL,
			status 		JSONB	NULL,
			tags 		JSONB	NULL,
			location 	POINT 	NULL,
			tenant		TEXT 	NOT NULL,	
			created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,			
			modified_on	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,	
			deleted     BOOLEAN DEFAULT FALSE,
			deleted_on  timestamp with time zone NULL,
			CONSTRAINT pkey_devices_unique PRIMARY KEY (device_id, sensor_id, deleted)
		);
	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) Close() {
	s.pool.Close()
}

func (s *Storage) UpdateDevice(ctx context.Context, device types.Device) error {
	data, _ := json.Marshal(device)
	profile, _ := json.Marshal(device.DeviceProfile)
	state, _ := json.Marshal(device.DeviceState)
	status, _ := json.Marshal(device.DeviceStatus)
	tags, _ := json.Marshal(device.Tags)

	var m map[string]any
	json.Unmarshal(data, &m)

	delete(m, "deviceID")
	delete(m, "sensorID")
	delete(m, "active")
	delete(m, "deviceProfile")
	delete(m, "deviceState")
	delete(m, "deviceStatus")
	delete(m, "tenant")
	delete(m, "tags")

	data, _ = json.Marshal(m)

	args := pgx.NamedArgs{
		"device_id": device.DeviceID,
		"sensor_id": device.SensorID,
		"active":    device.Active,
		"data":      string(data),
		"profile":   string(profile),
		"state":     string(state),
		"status":    string(status),
		"lat":       device.Location.Latitude,
		"lon":       device.Location.Longitude,
		"tags":      string(tags),
		"tenant":    device.Tenant,
	}

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET active = @active, data = @data, profile = @profile, state = @state, status = @status, tags = @tags, location = point(@lon,@lat), tenant = @tenant, modified_on = CURRENT_TIMESTAMP
		WHERE device_id = @device_id AND deleted = FALSE
	`, args)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) AddDevice(ctx context.Context, device types.Device) error {
	data, _ := json.Marshal(device)
	profile, _ := json.Marshal(device.DeviceProfile)
	state, _ := json.Marshal(device.DeviceState)
	status, _ := json.Marshal(device.DeviceStatus)
	tags, _ := json.Marshal(device.Tags)

	var m map[string]any
	json.Unmarshal(data, &m)

	delete(m, "deviceID")
	delete(m, "sensorID")
	delete(m, "active")
	delete(m, "deviceProfile")
	delete(m, "deviceState")
	delete(m, "deviceStatus")
	delete(m, "tenant")
	delete(m, "tags")

	data, _ = json.Marshal(m)

	args := pgx.NamedArgs{
		"device_id": device.DeviceID,
		"sensor_id": device.SensorID,
		"active":    device.Active,
		"data":      string(data),
		"profile":   string(profile),
		"state":     string(state),
		"status":    string(status),
		"lat":       device.Location.Latitude,
		"lon":       device.Location.Longitude,
		"tags":      string(tags),
		"tenant":    device.Tenant,
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO devices (device_id, sensor_id, active, data, profile, state, status, tags, location, tenant)
		VALUES (@device_id, @sensor_id, @active, @data, @profile, @state, @status, @tags, point(@lon,@lat), @tenant)
	`, args)
	if err != nil {
		return err
	}

	return nil
}

type ConditionFunc func(*Condition) *Condition

func (s *Storage) GetDevice(ctx context.Context, conditions ...ConditionFunc) (types.Device, error) {
	condition := &Condition{}
	for _, f := range conditions {
		f(condition)
	}

	args := condition.NamedArgs()
	where := condition.Where()

	var deviceID, sensorID, tenant string
	var location pgtype.Point
	var active bool
	var data, profile, state, status, tags json.RawMessage

	query := fmt.Sprintf(`
		SELECT device_id, sensor_id, active, data, profile, state, status, tags, location, tenant
		FROM devices
		WHERE %s AND deleted = FALSE
	`, where)

	err := s.pool.QueryRow(ctx, query, args).Scan(&deviceID, &sensorID, &active, &data, &profile, &state, &status, &tags, &location, &tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return types.Device{}, ErrNoRows
		}
		return types.Device{}, err
	}

	var errs []error

	var device types.Device
	errs = append(errs, json.Unmarshal(data, &device))
	errs = append(errs, json.Unmarshal(profile, &device.DeviceProfile))
	errs = append(errs, json.Unmarshal(state, &device.DeviceState))
	errs = append(errs, json.Unmarshal(status, &device.DeviceStatus))
	errs = append(errs, json.Unmarshal(tags, &device.Tags))

	device.DeviceID = deviceID
	device.SensorID = sensorID
	device.Active = active
	device.Tenant = tenant
	device.Location = types.Location{
		Latitude:  location.P.Y,
		Longitude: location.P.X,
	}

	return device, errors.Join(errs...)
}

func (s *Storage) QueryDevices(ctx context.Context, conditions ...ConditionFunc) (types.Collection[types.Device], error) {
	condition := &Condition{}
	for _, f := range conditions {
		f(condition)
	}

	args := condition.NamedArgs()
	where := condition.Where()

	var deviceID, sensorID, tenant string
	var location pgtype.Point
	var active bool
	var data, profile, state, status, tags json.RawMessage
	var count int64

	var offsetLimit string

	if condition.offset != nil {
		offsetLimit += fmt.Sprintf("OFFSET %d ", condition.Offset())
	}

	if condition.limit != nil {
		offsetLimit += fmt.Sprintf("LIMIT %d ", condition.Limit())
	}

	query := fmt.Sprintf(`
		SELECT device_id, sensor_id, active, data, profile, state, status, tags, location, tenant, count(*) OVER () AS count
		FROM devices
		WHERE %s AND deleted = FALSE		
		ORDER BY %s %s		
		%s
	`, where, condition.SortBy(), condition.SortOrder(), offsetLimit)

	rows, err := s.pool.Query(ctx, query, args)
	if err != nil {
		return types.Collection[types.Device]{}, err
	}

	devices := make([]types.Device, 0)

	_, err = pgx.ForEachRow(rows, []any{&deviceID, &sensorID, &active, &data, &profile, &state, &status, &tags, &location, &tenant, &count}, func() error {
		var errs []error
		var device types.Device

		errs = append(errs, json.Unmarshal(data, &device))
		errs = append(errs, json.Unmarshal(profile, &device.DeviceProfile))
		errs = append(errs, json.Unmarshal(state, &device.DeviceState))
		errs = append(errs, json.Unmarshal(status, &device.DeviceStatus))
		errs = append(errs, json.Unmarshal(tags, &device.Tags))

		device.DeviceID = deviceID
		device.SensorID = sensorID
		device.Active = active
		device.Tenant = tenant
		device.Location = types.Location{
			Latitude:  location.P.Y,
			Longitude: location.P.X,
		}
		devices = append(devices, device)

		return errors.Join(errs...)
	})
	if err != nil {
		return types.Collection[types.Device]{}, err
	}

	return types.Collection[types.Device]{
		Data:       devices,
		Count:      uint64(len(devices)),
		Limit:      uint64(condition.Limit()),
		Offset:     uint64(condition.Offset()),
		TotalCount: uint64(count),
	}, nil
}

func (s *Storage) UpdateStatus(ctx context.Context, deviceID, tenant string, deviceStatus types.DeviceStatus) error {
	status, _ := json.Marshal(deviceStatus)

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET status = @status
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"status":    string(status),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) UpdateState(ctx context.Context, deviceID, tenant string, deviceState types.DeviceState) error {
	state, _ := json.Marshal(deviceState)

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET state = @state
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"state":     string(state),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetTags(ctx context.Context, deviceID, tenant string) ([]types.Tag, error) {
	var tags json.RawMessage

	err := s.pool.QueryRow(ctx, `
		SELECT tags
		FROM devices
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
	}).Scan(&tags)
	if err != nil {
		return nil, err
	}

	var t []types.Tag
	err = json.Unmarshal(tags, &t)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (s *Storage) UpdateTags(ctx context.Context, deviceID, tenant string, tags []types.Tag) error {
	t, _ := json.Marshal(tags)

	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET tags = @tags
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"tags":      string(t),
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) SetTenant(ctx context.Context, deviceID, tenant string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET tenant = @tenant
		WHERE device_id = @device_id AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) SetActive(ctx context.Context, deviceID, tenant string, active bool) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE devices
		SET active = @active
		WHERE device_id = @device_id AND tenant = @tenant AND deleted = FALSE
	`, pgx.NamedArgs{
		"device_id": deviceID,
		"tenant":    tenant,
		"active":    active,
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) GetTenants(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT tenant
		FROM devices
		WHERE deleted = FALSE
	`)
	if err != nil {
		return []string{}, err
	}

	var tenants []string
	for rows.Next() {
		var tenant string
		rows.Scan(&tenant)
		tenants = append(tenants, tenant)
	}

	return tenants, nil
}
