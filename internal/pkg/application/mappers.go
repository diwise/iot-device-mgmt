package application

import (
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func MapStatus(status types.DeviceStatus) database.Status {
	return database.Status{
		DeviceID:     status.DeviceID,
		BatteryLevel: status.BatteryLevel,
		Status:       status.Code,
		Messages:     strings.Join(status.Messages, ","),
		Timestamp:    status.Timestamp,
	}
}

func MapToEnvModels(environments []database.Environment) []types.Environment {
	env := make([]types.Environment, 0)
	for _, e := range environments {
		env = append(env, types.Environment{ID: e.ID, Name: e.Name})
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
		DeviceID:    d.DeviceId,
		Name:        d.Name,
		Description: d.Description,
		Location: types.Location{
			Latitude:  d.Latitude,
			Longitude: d.Longitude,
		},
		Environment: env,
		Types:       lwm2mTypes(d.Types),
		SensorType: types.SensorType{
			ID:       d.SensorType.ID,
			Name:     d.SensorType.Name,
			Interval: d.SensorType.Interval,
		},
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
