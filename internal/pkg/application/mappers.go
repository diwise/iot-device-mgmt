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

func MapToModel(d database.Device, s database.Status) types.Device {
	env := d.Environment.Name
	t := d.Tenant.Name

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
		Types: MapTo(d.Types, func(e database.Lwm2mType) string {
			return e.Type
		}),
		SensorType: types.SensorType{
			ID:          d.SensorType.ID,
			Name:        d.SensorType.Name,
			Description: d.SensorType.Description,
			Interval:    d.SensorType.Interval,
		},
		LastObserved: d.LastObserved,
		Active:       d.Active,
		Tenant:       t,
		Status: types.DeviceStatus{
			BatteryLevel: s.BatteryLevel,
			Code:         s.Status,
			Timestamp:    s.Timestamp,
		},
		Interval: d.Interval,
	}

	if len(s.Messages) > 0 {
		dev.Status.Messages = strings.Split(s.Messages, ",")
	}

	return dev
}

func MapTo[TFrom any, TTo any](data []TFrom, f func(TFrom) TTo) []TTo {
	result := make([]TTo, 0)
	for _, v := range data {
		result = append(result, f(v))
	}
	return result
}
