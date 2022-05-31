package application

import (
	"context"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
)

type DeviceManagement interface {
	GetDevice(context.Context, string) (database.Device, error)
	GetDeviceFromEUI(context.Context, string) (database.Device, error)
	ListAllDevices(ctx context.Context) ([]database.Device, error)
}

func New(db database.Datastore) DeviceManagement {
	a := &app{
		db: db,
	}

	return a
}

type app struct {
	db database.Datastore
}

func (a *app) GetDevice(ctx context.Context, deviceID string) (database.Device, error) {
	device, err := a.db.GetDeviceFromID(deviceID)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (a *app) GetDeviceFromEUI(ctx context.Context, devEUI string) (database.Device, error) {
	device, err := a.db.GetDeviceFromDevEUI(devEUI)
	if err != nil {
		return nil, err
	}

	return device, nil
}

func (a *app) ListAllDevices(ctx context.Context) ([]database.Device, error) {
	devices, err := a.db.GetAll()
	if err != nil {
		return nil, err
	}

	return devices, nil
}
