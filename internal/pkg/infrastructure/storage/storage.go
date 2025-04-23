package storage

import (
	"context"
	"errors"
	"fmt"

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



		CREATE TABLE IF NOT EXISTS device_profiles (
			device_profile_id	TEXT NOT NULL,
			name 				TEXT NULL,
			decoder 			TEXT NOT NULL,			
			description			TEXT NULL,
			interval 			NUMERIC NOT NULL DEFAULT 3600,
			created_on  		timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
		);

		CREATE TABLE IF NOT EXISTS device_profiles_types (
			device_profile_type_id	TEXT NOT NULL,
			name 					TEXT NULL,
			created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT pkey_device_profiles_types PRIMARY KEY (device_profile_type_id)
		);

		CREATE TABLE IF NOT EXISTS device_profiles_device_profiles_types (
			device_profile_id 		TEXT NOT NULL,
			device_profile_type_id	TEXT NOT NULL,			
			created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT pkey_device_profiles_device_profiles_types PRIMARY KEY (device_profile_id, device_profile_type_id)
		);

		CREATE TABLE IF NOT EXISTS device_device_profile_types (
			device_id 				TEXT NOT NULL,
			device_profile_type_id	TEXT NOT NULL,			
			created_on  			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT pkey_device_device_profile_types PRIMARY KEY (device_id, device_profile_type_id)
		);

		CREATE TABLE IF NOT EXISTS device_status (
			time 			timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			device_id		TEXT 	NOT NULL,
			battery_level 	NUMERIC NULL,
			rssi 			NUMERIC NULL,
			snr 			NUMERIC NULL,
			fq 				NUMERIC NULL,
			sf 				NUMERIC NULL,
			dr 				NUMERIC NULL,
			created_on  	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT pkey_device_status PRIMARY KEY (time, device_id)
		);

		CREATE TABLE IF NOT EXISTS device_alarms (
			device_id		TEXT NOT NULL,
			type			TEXT NOT NULL,
			description		TEXT NULL,
			created_on  	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
		);



		CREATE TABLE IF NOT EXISTS alarms (
    		alarm_id 	VARCHAR(255),
    		alarm_type 	VARCHAR(100) NOT NULL, 
    		description TEXT,                
    		observed_at timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    		ref_id 		VARCHAR(255) NOT NULL,    
    		severity 	INT NOT NULL,          
    		tenant 		VARCHAR(255) NOT NULL  ,  
			created_on  timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,			
			modified_on	timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,	
			deleted     BOOLEAN DEFAULT FALSE,
			deleted_on  timestamp with time zone NULL,
			CONSTRAINT pkey_alarms_unique PRIMARY KEY (alarm_id, deleted)
		);

		CREATE INDEX IF NOT EXISTS devices_name_idx ON devices ((data ->> 'name'));
		CREATE INDEX IF NOT EXISTS devices_tenant_deleted_idx ON devices (tenant) WHERE NOT deleted;

	`)
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) Close() {
	s.pool.Close()
}
