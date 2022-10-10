package application

import (
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
)

type Device struct {
	DevEUI       string    `json:"devEUI"`
	DeviceId     string    `json:"deviceID"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Location     Location  `json:"location"`
	Environment  string    `json:"environment"`
	Types        []string  `json:"types"`
	SensorType   string    `json:"sensor_type"`
	LastObserved time.Time `json:"last_observed"`
	Active       bool      `json:"active"`
	Tenant       string    `json:"tenant"`
	Status       Status    `json:"status"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitue   float64 `json:"altitude"`
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

func MapToModel(d database.Device, s database.Status) Device {
	env := d.Environment.Name
	t := d.Tenant.Name

	types := func(x []database.Lwm2mType) []string {
		t := make([]string, 0)
		for _, l := range x {
			t = append(t, l.Type)
		}
		return t
	}

	dev := Device{
		DevEUI:      d.DevEUI,
		DeviceId:    d.DeviceId,
		Name:        d.Name,
		Description: d.Description,
		Location: Location{
			Latitude:  d.Latitude,
			Longitude: d.Longitude,
		},
		Environment:  env,
		Types:        types(d.Types),
		SensorType:   d.SensorType,
		LastObserved: d.LastObserved,
		Active:       d.Active,
		Tenant:       t,
		Status: Status{
			BatteryLevel: s.BatteryLevel,
			Code:         s.Status,
			Timestamp:    s.Timestamp,
		},
	}

	if len(s.Messages) > 0 {
		dev.Status.Messages = strings.Split(s.Messages, ",")
	}

	return dev
}

type Status struct {
	BatteryLevel int      `json:"batteryLevel"`
	Code         int      `json:"statusCode"`
	Messages     []string `json:"statusMessages,omitempty"`
	Timestamp    string   `json:"timestamp"`
}
