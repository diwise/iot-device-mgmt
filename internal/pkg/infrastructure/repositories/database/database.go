package database

import (
	"encoding/csv"
	"fmt"
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
	CreateDevice(devEUI, deviceId, name, description, environment, sensorType string, latitude, longitude float64, types []string, active bool) (Device, error)
	UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) error
	GetAll() ([]Device, error)

	ListEnvironments() ([]Environment, error)

	Seed(f string) error
}

type store struct {
	db     *gorm.DB
	logger zerolog.Logger
}

// ConnectorFunc is used to inject a database connection method into NewDatabaseConnection
type ConnectorFunc func() (*gorm.DB, zerolog.Logger, error)

func NewDatabaseConnection(connect ConnectorFunc) (Datastore, error) {
	impl, logger, err := connect()
	if err != nil {
		return nil, err
	}

	err = impl.AutoMigrate(&Device{}, &Lwm2mType{}, &Environment{})
	if err != nil {
		return nil, err
	}

	return &store{
		db:     impl,
		logger: logger,
	}, nil
}

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

func (s store) Seed(seedFile string) error {
	devicesFile, err := os.Open(seedFile)
	if err != nil {
		return err
	}
	defer devicesFile.Close()

	r := csv.NewReader(devicesFile)
	r.Comma = ';'

	knownDevices, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("failed to read csv data from file: %s", err.Error())
	}

	s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "name"}},
		DoNothing: true,
	}).CreateInBatches([]Environment{
		{Name: "air"},
		{Name: "ground"},
		{Name: "water"},
		{Name: "indoors"},
		{Name: "lifebuoy"},
	}, 5)

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
		s.db.First(&environment, "name=?", d[4])

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
	result := s.db.Preload("Types").Preload("Environment").First(&d, "dev_eui=?", eui)

	return d, result.Error
}

func (s store) GetDeviceFromID(deviceID string) (Device, error) {
	var d Device
	result := s.db.Preload("Types").Preload("Environment").First(&d, "device_id=?", deviceID)

	return d, result.Error
}

func (s store) UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) error {
	result := s.db.Model(&Device{}).Where("device_id = ?", deviceID).Update("last_observed", timestamp)
	return result.Error
}

func (s store) GetAll() ([]Device, error) {
	var devices []Device
	err := s.db.Debug().Preload("Types").Preload("Environment").Find(&devices).Error
	if err != nil {
		return nil, err
	}

	return devices, err
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

func (s store) CreateDevice(devEUI, deviceId, name, description, environment, sensorType string, latitude, longitude float64, types []string, active bool) (Device, error) {
	var env Environment
	s.db.First(&env, "name=?", environment)

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
	}

	err := s.db.Create(&d).Error

	return d, err
}

func (s store) ListEnvironments() ([]Environment, error) {
	var env []Environment
	err := s.db.Find(&env).Error

	return env, err
}
