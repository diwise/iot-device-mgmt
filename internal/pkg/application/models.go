package application

import (
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
)

type Device struct {
	DevEUI       string    `json:"devEUI"`
	DeviceId     string    `json:"deviceID"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	Environment  string    `json:"environment"`
	Types        []string  `json:"types"`
	SensorType   string    `json:"sensor_type"`
	LastObserved time.Time `json:"last_observed"`
	Active       bool      `json:"active"`
	Tenant       string    `json:"tenant"`
}

type Environment struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func MapToEnvModels(environments []database.Environment) []Environment {
	env := make([]Environment, 0)
	for _, e := range environments {
		env = append(env, Environment{ID: e.ID, Name: e.Name})
	}
	return env
}

func MapToModels(d []database.Device) []Device {
	devices := make([]Device, 0)
	for _, db := range d {
		devices = append(devices, MapToModel(db))
	}
	return devices
}

func MapToModel(d database.Device) Device {
	env := d.Environment.Name
	t := d.Tenant.Name

	types := func(x []database.Lwm2mType) []string {
		t := make([]string, 0)
		for _, l := range x {
			t = append(t, l.Type)
		}
		return t
	}

	return Device{
		DevEUI:       d.DevEUI,
		DeviceId:     d.DeviceId,
		Name:         d.Name,
		Description:  d.Description,
		Latitude:     d.Latitude,
		Longitude:    d.Longitude,
		Environment:  env,
		Types:        types(d.Types),
		SensorType:   d.SensorType,
		LastObserved: d.LastObserved,
		Active:       d.Active,
		Tenant:       t,
	}
}

type StatusMessage struct {
	DeviceID  string  `json:"deviceID"`
	Error     *string `json:"error,omitempty"`
	Status    Status  `json:"status"`
	Timestamp string  `json:"timestamp"`
}

type Status struct {
	Code     int      `json:"statusCode"`
	Messages []string `json:"statusMessages,omitempty"`
}