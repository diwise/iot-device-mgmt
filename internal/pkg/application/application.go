package application

import (
	"context"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
)

//go:generate moq -rm -out application_mock.go . DeviceManagement

type DeviceManagement interface {
	GetDevice(context.Context, string) (Device, error)
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]interface{}) (Device, error)
	CreateDevice(context.Context, Device) error
	GetDeviceFromEUI(context.Context, string) (Device, error)
	ListAllDevices(context.Context) ([]Device, error)
	UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) error
	ListEnvironments(context.Context) ([]Environment, error)
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

func (a *app) GetDevice(ctx context.Context, deviceID string) (Device, error) {
	device, err := a.db.GetDeviceFromID(deviceID)
	if err != nil {
		return Device{}, err
	}

	return MapToModel(device), nil
}

func (a *app) GetDeviceFromEUI(ctx context.Context, devEUI string) (Device, error) {
	device, err := a.db.GetDeviceFromDevEUI(devEUI)
	if err != nil {
		return Device{}, err
	}

	return MapToModel(device), nil
}

func (a *app) ListAllDevices(ctx context.Context) ([]Device, error) {
	devices, err := a.db.GetAll()
	if err != nil {
		return nil, err
	}

	return MapToModels(devices), nil
}

func (a *app) UpdateLastObservedOnDevice(deviceID string, timestamp time.Time) error {
	err := a.db.UpdateLastObservedOnDevice(deviceID, timestamp)
	if err != nil {
		return err
	}

	return nil
}

func (a *app) UpdateDevice(ctx context.Context, deviceID string, fields map[string]interface{}) (Device, error) {
	d, err := a.db.UpdateDevice(deviceID, fields)
	return MapToModel(d), err
}

func (a *app) CreateDevice(ctx context.Context, d Device) error {
	_, err := a.db.CreateDevice(d.DevEUI, d.DeviceId, d.Name, d.Description, d.Environment, d.SensorType, d.Tenant, d.Latitude, d.Longitude, d.Types, d.Active)
	return err
}

func (a *app) ListEnvironments(context.Context) ([]Environment, error) {
	env, err := a.db.ListEnvironments()
	if err != nil {
		return nil, err
	}
	return MapToEnvModels(env), nil
}
