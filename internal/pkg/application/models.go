package application

import (
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

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

func MapToModel(d database.Device, s database.Status) types.Device {
	env := d.Environment.Name
	t := d.Tenant.Name

	lwm2mTypes := func(x []database.Lwm2mType) []string {
		t := make([]string, 0)
		for _, l := range x {
			t = append(t, l.Type)
		}
		return t
	}

	dev := types.Device{
		DevEUI:      d.DevEUI,
		DeviceId:    d.DeviceId,
		Name:        d.Name,
		Description: d.Description,
		Location: types.Location{
			Latitude:  d.Latitude,
			Longitude: d.Longitude,
		},
		Environment:  env,
		Types:        lwm2mTypes(d.Types),
		SensorType:   d.SensorType,
		LastObserved: d.LastObserved,
		Active:       d.Active,
		Tenant:       t,
		Status: types.DeviceStatus{
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
