package database

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

//go:generate moq -rm -out database_mock.go . Datastore

type Datastore interface {
	GetDeviceFromDevEUI(eui string) (Device, error)
	GetDeviceFromID(deviceID string) (Device, error)
	UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) (Device, error)
	GetAll() ([]Device, error)
}

type database struct {
	log          zerolog.Logger
	devicesByEUI map[string]*device
	devicesByID  map[string]*device
}

func SetUpNewDatabase(log zerolog.Logger, devicesFile io.Reader) (Datastore, error) {
	r := csv.NewReader(devicesFile)
	r.Comma = ';'

	knownDevices, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read csv data from file: %s", err.Error())
	}

	db := &database{
		log:          log,
		devicesByEUI: map[string]*device{},
		devicesByID:  map[string]*device{},
	}

	// Create a set of allowed environments from a slice of allowed envs so that
	// we can validate and provide helpful diagnostics if config is wrong
	allowedEnvironments := []string{"air", "ground", "water", "indoors", "lifebuoy"}
	setOfAllowedEnvironments := map[string]bool{}
	for _, env := range allowedEnvironments {
		setOfAllowedEnvironments[env] = true
	}

	for idx, d := range knownDevices {
		if idx == 0 {
			// Skip the CSV header
			continue
		}

		devEUI := d[0]
		deviceID := d[1]

		_, ok := db.devicesByEUI[devEUI]
		if ok {
			return nil, fmt.Errorf("duplicate devEUI %s found on line %d in devices config", devEUI, (idx + 1))
		}

		_, ok = db.devicesByID[deviceID]
		if ok {
			return nil, fmt.Errorf("duplicate device id %s found on line %d in devices config", deviceID, (idx + 1))
		}

		lat, err := strconv.ParseFloat(d[2], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse latitude for device %s: %s", devEUI, err.Error())
		}
		lon, err := strconv.ParseFloat(d[3], 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse longitude for device %s: %s", devEUI, err.Error())
		}

		environment := d[4]
		if !setOfAllowedEnvironments[environment] {
			return nil, fmt.Errorf("bad environment specified for device %s on line %d in config (\"%s\" not in %v)", devEUI, (idx + 1), environment, allowedEnvironments)
		}

		types := strings.Split(d[5], ",")

		sensorType := d[6]

		dev := &device{
			Identity:    d[1],
			Latitude:    lat,
			Longitude:   lon,
			Environment: environment,
			Types:       types,
			SensorType:  sensorType,
		}

		db.devicesByEUI[devEUI] = dev
		db.devicesByID[deviceID] = dev
	}

	db.log.Info().Msgf("loaded %d devices from configuration file", len(db.devicesByEUI))

	return db, nil
}

func (db *database) GetDeviceFromDevEUI(eui string) (Device, error) {

	device, ok := db.devicesByEUI[eui]
	if !ok {
		return nil, fmt.Errorf("no matching devices found with devEUI %s", eui)
	}

	return device, nil
}

func (db *database) GetDeviceFromID(deviceID string) (Device, error) {

	device, ok := db.devicesByID[deviceID]
	if !ok {
		return nil, fmt.Errorf("no matching devices found with id %s", deviceID)
	}

	return device, nil
}

func (db *database) GetAll() ([]Device, error) {
	var devices []Device
	for _, v := range db.devicesByID {
		devices = append(devices, v)
	}
	return devices, nil
}

func (db *database) UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) (Device, error) {
	device, ok := db.devicesByID[deviceID]
	if !ok {
		return nil, fmt.Errorf("no matching devices found with id %s", deviceID)
	}

	if device.LastObserved.IsZero() {
		device.LastObserved = timestamp
		return device, nil
	}

	if device.LastObserved.After(timestamp) {
		db.log.Info().Msgf("lastObserved %s is more recent than incoming time %s, ignoring", device.LastObserved.Format(time.RFC3339), timestamp.Format(time.RFC3339))
		return nil, nil
	}

	return device, nil
}

type Device interface {
	ID() string
}

type device struct {
	Identity     string    `json:"id"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Environment  string    `json:"environment"`
	Types        []string  `json:"types"`
	SensorType   string    `json:"sensorType"`
	LastObserved time.Time `json:"lastObserved"`
}

func (d device) ID() string {
	return d.Identity
}
