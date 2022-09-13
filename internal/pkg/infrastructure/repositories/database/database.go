package database

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

//go:generate moq -rm -out database_mock.go . Datastore

type Datastore interface {
	GetDeviceFromDevEUI(eui string) (Device, error)
	GetDeviceFromID(deviceID string) (Device, error)
	UpdateDevice(deviceID string, fields map[string]interface{}) (Device, error)
	CreateDevice(devEUI, deviceId, name, description, environment, sensorType, tenant string, latitude, longitude float64, types []string, active bool) (Device, error)
	UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) error
	GetAll(tenants []string) ([]Device, error)

	ListEnvironments() ([]Environment, error)

	Seed(r io.Reader) error
}

type store struct {
	db     *gorm.DB
	logger zerolog.Logger
}

// ConnectorFunc is used to inject a database connection method into NewDatabaseConnection
type ConnectorFunc func() (*gorm.DB, zerolog.Logger, error)

// NewPostgreSQLConnector opens a connection to a postgresql database
func NewPostgreSQLConnector(log zerolog.Logger) ConnectorFunc {
	dbHost := os.Getenv("DIWISE_SQLDB_HOST")
	username := os.Getenv("DIWISE_SQLDB_USER")
	dbName := os.Getenv("DIWISE_SQLDB_NAME")
	password := os.Getenv("DIWISE_SQLDB_PASSWORD")
	sslMode := env.GetVariableOrDefault(log, "DIWISE_SQLDB_SSLMODE", "require")

	dbURI := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=%s password=%s", dbHost, username, dbName, sslMode, password)

	return func() (*gorm.DB, zerolog.Logger, error) {
		sublogger := log.With().Str("host", dbHost).Str("database", dbName).Logger()

		for {
			sublogger.Info().Msg("connecting to database host")

			db, err := gorm.Open(postgres.Open(dbURI), &gorm.Config{
				Logger: logger.New(
					&sublogger,
					logger.Config{
						SlowThreshold:             time.Second,
						LogLevel:                  logger.Info,
						IgnoreRecordNotFoundError: false,
						Colorful:                  false,
					},
				),
			})
			if err != nil {
				sublogger.Fatal().Err(err).Msg("failed to connect to database")
				time.Sleep(3 * time.Second)
			} else {
				return db, sublogger, nil
			}
		}
	}
}

// NewSQLiteConnector opens a connection to a local sqlite database
func NewSQLiteConnector(log zerolog.Logger) ConnectorFunc {
	return func() (*gorm.DB, zerolog.Logger, error) {
		db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})

		if err == nil {
			db.Exec("PRAGMA foreign_keys = ON")
		}

		return db, log, err
	}
}

func NewDatabaseConnection(connect ConnectorFunc) (Datastore, error) {
	impl, logger, err := connect()
	if err != nil {
		return nil, err
	}

	err = impl.AutoMigrate(&Device{}, &Lwm2mType{}, &Environment{}, &Tenant{})
	if err != nil {
		return nil, err
	}

	return &store{
		db:     impl,
		logger: logger,
	}, nil
}

func (s store) Seed(seedFileReader io.Reader) error {

	r := csv.NewReader(seedFileReader)
	r.Comma = ';'

	knownDevices, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read csv data from file: %s", err.Error())
	}

	devices := []Device{}

	for idx, d := range knownDevices {
		if idx == 0 {
			// Skip the CSV header
			continue
		}

		devEUI := d[0]
		deviceID := d[1]

		lat, err := strconv.ParseFloat(d[2], 64)
		if err != nil {
			return fmt.Errorf("failed to parse latitude for device %s: %s", devEUI, err.Error())
		}
		lon, err := strconv.ParseFloat(d[3], 64)
		if err != nil {
			return fmt.Errorf("failed to parse longitude for device %s: %s", devEUI, err.Error())
		}

		var environment Environment
		result := s.db.First(&environment, "name=?", d[4])
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			newEnvironment := Environment{Name: d[4]}
			s.db.Create(&newEnvironment)
			environment = newEnvironment
		}

		types := []Lwm2mType{}
		ts := strings.Split(d[5], ",")

		for _, t := range ts {
			types = append(types, Lwm2mType{Type: t})
		}

		sensorType := d[6]

		name := d[7]

		description := d[8]

		active, err := strconv.ParseBool(d[9])
		if err != nil {
			return fmt.Errorf("failed to parse active for device %s: %s", devEUI, err.Error())
		}

		var tenant Tenant
		result = s.db.First(&tenant, "name=?", d[10])
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			newTenant := Tenant{Name: d[10]}
			s.db.Create(&newTenant)
			tenant = newTenant
		}

		d := Device{
			DevEUI:      devEUI,
			DeviceId:    deviceID,
			Name:        name,
			Description: description,
			Latitude:    lat,
			Longitude:   lon,
			Environment: environment,
			Types:       types,
			SensorType:  sensorType,
			Active:      active,
			Tenant:      tenant,
		}

		devices = append(devices, d)
	}

	result := s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_id"}},
		UpdateAll: true,
	}).Create(devices)

	return result.Error
}

func (s store) GetDeviceFromDevEUI(eui string) (Device, error) {
	var d Device
	result := s.db.Preload("Types").Preload("Environment").Preload("Tenant").First(&d, "dev_eui=?", eui)

	return d, result.Error
}

func (s store) GetDeviceFromID(deviceID string) (Device, error) {
	var d Device
	result := s.db.Preload("Types").Preload("Environment").Preload("Tenant").First(&d, "device_id=?", deviceID)

	return d, result.Error
}

func (s store) UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) error {
	result := s.db.Model(&Device{}).Where("device_id = ?", deviceID).Update("last_observed", timestamp)
	return result.Error
}

func (s store) getTenantByName(tenantName string) (Tenant, error) {
	var tenant Tenant

	err := s.db.First(&tenant, "name = ?", tenantName).Error
	if err != nil {
		return Tenant{}, err
	}

	return tenant, nil
}

func (s store) GetAll(tenants []string) ([]Device, error) {
	var deviceList []Device

	for _, t := range tenants {
		tenant, err := s.getTenantByName(t)
		if err != nil {
			return nil, err
		}

		var devices []Device

		err = s.db.Preload("Types").Preload("Environment").Preload("Tenant").Find(&devices, "tenant_id = ?", tenant.ID).Error
		if err != nil {
			return nil, err
		}

		deviceList = append(deviceList, devices...)
	}

	return deviceList, nil
}

func (s store) UpdateDevice(deviceID string, fields map[string]interface{}) (Device, error) {
	d, err := s.GetDeviceFromID(deviceID)
	if err != nil {
		return Device{}, err
	}

	result := s.db.Model(&d).Select("name", "description", "latitude", "longitude", "active").Updates(fields)
	if result.Error != nil {
		return Device{}, result.Error
	}

	return s.GetDeviceFromID(deviceID)
}

func (s store) CreateDevice(devEUI, deviceId, name, description, environment, sensorType, tenant string, latitude, longitude float64, types []string, active bool) (Device, error) {
	var env Environment
	s.db.First(&env, "name=?", environment)

	var t Tenant
	s.db.First(&tenant, "name=?", tenant)

	lwm2mTypes := []Lwm2mType{}
	for _, t := range types {
		lwm2mTypes = append(lwm2mTypes, Lwm2mType{Type: t})
	}

	d := Device{
		DevEUI:      devEUI,
		DeviceId:    deviceId,
		Name:        name,
		Description: description,
		SensorType:  sensorType,
		Latitude:    latitude,
		Longitude:   longitude,
		Active:      active,
		Environment: env,
		Types:       lwm2mTypes,
		Tenant:      t,
	}

	err := s.db.Create(&d).Error

	return d, err
}

func (s store) ListEnvironments() ([]Environment, error) {
	var env []Environment
	err := s.db.Find(&env).Error

	return env, err
}
