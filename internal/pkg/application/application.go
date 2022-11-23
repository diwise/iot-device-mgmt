package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/events"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/webevents"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

//go:generate moq -rm -out application_mock.go . App

type App interface {
	Start()
	Stop()
	Handle(ctx context.Context, ds types.DeviceStatus) error

	GetDevice(ctx context.Context, deviceID string) (types.Device, error)
	GetDeviceByEUI(ctx context.Context, eui string) (types.Device, error)
	GetDevices(ctx context.Context, tenants []string) ([]types.Device, error)
	CreateDevice(ctx context.Context, device types.Device) error
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]interface{}) (types.Device, error)

	SetStatus(ctx context.Context, deviceID string, message types.DeviceStatus) error

	GetTenants(ctx context.Context) ([]string, error)
	GetEnvironments(ctx context.Context) ([]types.Environment, error)
}

type app struct {
	store       database.Datastore
	eventSender events.EventSender
	webEvents   webevents.WebEvents
}

func New(s database.Datastore, e events.EventSender, we webevents.WebEvents) App {
	return &app{
		store:       s,
		eventSender: e,
		webEvents:   we,
	}
}

func (a *app) Start() {}
func (a *app) Stop() {
	a.webEvents.Shutdown()
}

func (a *app) Handle(ctx context.Context, ds types.DeviceStatus) error {
	log := logging.GetFromContext(ctx)
	deviceID := ds.DeviceID
	timestamp, err := time.Parse(time.RFC3339Nano, ds.Timestamp)
	if err != nil {
		return fmt.Errorf("unable to parse timestamp from deviceStatus, %w", err)
	}

	err = a.store.UpdateLastObservedOnDevice(deviceID, timestamp)
	if err != nil {
		return fmt.Errorf("could not update last observed on device %s, %w", deviceID, err)
	}
	err = a.store.SetStatusIfChanged(MapStatus(ds))
	if err != nil {
		return fmt.Errorf("could not update status for device %s %w", deviceID, err)
	}

	d, err := a.GetDevice(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("could not fetch device %s from datastore, %w", deviceID, err)
	}
	err = a.webEvents.Publish("lastObservedUpdated", d)
	if err != nil {
		log.Error().Err(err).Msgf("could not publish web event for device %s", deviceID)
	}
	err = a.eventSender.Send(ctx, deviceID, ds)
	if err != nil {
		return fmt.Errorf("could not send status event for device %s, %w", deviceID, err)
	}

	return nil
}

func (a *app) GetDevice(ctx context.Context, deviceID string) (types.Device, error) {
	d, err := a.store.GetDeviceFromID(deviceID)
	if err != nil {
		return types.Device{}, err
	}

	status, err := a.store.GetLatestStatus(deviceID)
	if err != nil {
		return types.Device{}, err
	}

	return MapToModel(d, status), nil
}

func (a *app) GetDeviceByEUI(ctx context.Context, eui string) (types.Device, error) {
	d, err := a.store.GetDeviceFromDevEUI(eui)
	if err != nil {
		return types.Device{}, err
	}

	status, err := a.store.GetLatestStatus(d.DeviceId)
	if err != nil {
		return types.Device{}, err
	}

	return MapToModel(d, status), nil
}

func (a *app) GetDevices(ctx context.Context, tenants []string) ([]types.Device, error) {
	devices, err := a.store.GetAll(tenants...)
	if err != nil {
		return nil, err
	}

	models := make([]types.Device, 0)

	for _, d := range devices {
		status, err := a.store.GetLatestStatus(d.DeviceId)
		if err != nil {
			return nil, err
		}
		models = append(models, MapToModel(d, status))
	}

	return models, nil
}

func (a *app) CreateDevice(ctx context.Context, d types.Device) error {
	device, err := a.store.CreateDevice(d.DevEUI, d.DeviceID, d.Name, d.Description, d.Environment, d.SensorType.Name, d.Tenant, d.Location.Latitude, d.Location.Longitude, d.Types, d.Active)
	if err != nil {
		return err
	}

	err = a.webEvents.Publish("deviceCreated", MapToModel(device, database.Status{}))
	if err != nil {
		log := logging.GetFromContext(ctx)
		log.Error().Err(err).Msg("could not publish web event")
	}

	return nil
}

func (a *app) UpdateDevice(ctx context.Context, deviceID string, fields map[string]interface{}) (types.Device, error) {
	device, err := a.store.UpdateDevice(deviceID, fields)
	if err != nil {
		return types.Device{}, err
	}

	m := MapToModel(device, database.Status{})

	err = a.webEvents.Publish("deviceUpdated", MapToModel(device, database.Status{}))
	if err != nil {
		log := logging.GetFromContext(ctx)
		log.Error().Err(err).Msg("could not publish web event")
	}

	return m, nil
}

func (a *app) GetTenants(ctx context.Context) ([]string, error) {
	t := a.store.GetAllTenants()
	return t, nil
}

func (a *app) GetEnvironments(ctx context.Context) ([]types.Environment, error) {
	environments, err := a.store.ListEnvironments()
	if err != nil {
		return nil, err
	}

	return MapTo(environments, func(e database.Environment) types.Environment {
		return types.Environment{ID: e.ID, Name: e.Name}
	}), nil
}

func (a *app) SetStatus(ctx context.Context, deviceID string, message types.DeviceStatus) error {
	s := database.Status{
		DeviceID:     deviceID,
		BatteryLevel: message.BatteryLevel,
		Status:       message.Code,
		Messages:     strings.Join(message.Messages, ","),
		Timestamp:    message.Timestamp,
	}

	return a.store.SetStatusIfChanged(s)
}
