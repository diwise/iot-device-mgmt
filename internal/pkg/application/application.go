package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexandrevicenzi/go-sse"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"golang.org/x/sys/unix"

	cloudevents "github.com/cloudevents/sdk-go/v2"
)

//go:generate moq -rm -out application_mock.go . DeviceManagement

type DeviceManagement interface {
	GetDevice(context.Context, string) (Device, error)
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]interface{}) (Device, error)
	CreateDevice(context.Context, Device) error
	GetDeviceFromEUI(context.Context, string) (Device, error)
	ListAllDevices(context.Context, []string) ([]Device, error)
	UpdateLastObservedOnDevice(ctx context.Context, deviceID string, timestamp time.Time) error
	ListEnvironments(context.Context) ([]Environment, error)
	NotifyStatus(ctx context.Context, deviceID string, message Status) error
	SetStatusIfChanged(ctx context.Context, deviceID string, message Status) error
}

func New(db database.Datastore, cfg *Config, sseServer *sse.Server) DeviceManagement {
	a := &app{
		db:          db,
		subscribers: make(map[string][]SubscriberConfig),
		sse:         sseServer,
	}

	if cfg != nil {
		for _, s := range cfg.Notifications {
			a.subscribers[s.Type] = s.Subscribers
		}
	}

	return a
}

type app struct {
	db          database.Datastore
	subscribers map[string][]SubscriberConfig
	sse         *sse.Server
}

func (a *app) GetDevice(ctx context.Context, deviceID string) (Device, error) {
	device, err := a.db.GetDeviceFromID(deviceID)
	if err != nil {
		return Device{}, err
	}

	sm, _ := a.db.GetLatestStatus(device.DeviceId)

	return MapToModel(device, sm), nil
}

func (a *app) GetDeviceFromEUI(ctx context.Context, devEUI string) (Device, error) {
	device, err := a.db.GetDeviceFromDevEUI(devEUI)
	if err != nil {
		return Device{}, err
	}

	sm, _ := a.db.GetLatestStatus(device.DeviceId)

	return MapToModel(device, sm), nil
}

func (a *app) ListAllDevices(ctx context.Context, allowedTenants []string) ([]Device, error) {

	devices, err := a.db.GetAll(allowedTenants)
	if err != nil {
		return nil, err
	}

	models := make([]Device, 0)

	for _, d := range devices {
		sm, _ := a.db.GetLatestStatus(d.DeviceId) // TODO: select n+1
		models = append(models, MapToModel(d, sm))
	}

	return models, nil
}

func (a *app) UpdateLastObservedOnDevice(ctx context.Context, deviceID string, timestamp time.Time) error {
	err := a.db.UpdateLastObservedOnDevice(deviceID, timestamp)
	if err != nil {
		return err
	}

	d, err := a.GetDevice(ctx, deviceID)
	if err != nil {
		return err
	}

	return a.sendMessage("lastObservedUpdated", d)
}

func (a *app) UpdateDevice(ctx context.Context, deviceID string, fields map[string]interface{}) (Device, error) {
	d, err := a.db.UpdateDevice(deviceID, fields)

	if err != nil {
		return Device{}, err
	}

	m := MapToModel(d, database.Status{})

	return m, a.sendMessage("deviceUpdated", m)
}

func (a *app) CreateDevice(ctx context.Context, d Device) error {
	device, err := a.db.CreateDevice(d.DevEUI, d.DeviceId, d.Name, d.Description, d.Environment, d.SensorType, d.Tenant, d.Location.Latitude, d.Location.Longitude, d.Types, d.Active)

	if err != nil {
		return err
	}

	return a.sendMessage("deviceCreated", MapToModel(device, database.Status{}))
}

func (a *app) ListEnvironments(context.Context) ([]Environment, error) {
	env, err := a.db.ListEnvironments()
	if err != nil {
		return nil, err
	}
	return MapToEnvModels(env), nil
}

func (a *app) NotifyStatus(ctx context.Context, deviceID string, message Status) error {
	if s, ok := a.subscribers["diwise.statusmessage"]; !ok || len(s) == 0 {
		return nil
	}

	var err error

	c, err := cloudevents.NewClientHTTP()
	if err != nil {
		return err
	}

	event := cloudevents.NewEvent()
	if timestamp, err := time.Parse(time.RFC3339, message.Timestamp); err == nil {
		event.SetID(fmt.Sprintf("%s:%d", deviceID, timestamp.Unix()))
		event.SetTime(timestamp)
	} else {
		return err
	}

	eventData := struct {
		DeviceID     string   `json:"deviceID"`
		BatteryLevel int      `json:"batteryLevel"`
		Status       int      `json:"statusCode"`
		Messages     []string `json:"statusMessages"`
		Timestamp    string   `json:"timestamp"`
	}{
		DeviceID:     deviceID,
		BatteryLevel: message.BatteryLevel,
		Status:       message.Code,
		Messages:     message.Messages,
		Timestamp:    message.Timestamp,
	}

	event.SetSource("github.com/diwise/iot-device-mgmt")
	event.SetType("diwise.statusmessage")
	event.SetData(cloudevents.ApplicationJSON, eventData)

	logger := logging.GetFromContext(ctx)

	for _, s := range a.subscribers["diwise.statusmessage"] {
		ctxWithTarget := cloudevents.ContextWithTarget(ctx, s.Endpoint)

		result := c.Send(ctxWithTarget, event)
		if cloudevents.IsUndelivered(result) || errors.Is(result, unix.ECONNREFUSED) {
			logger.Error().Err(result).Msgf("faild to send event to %s", s.Endpoint)
			err = fmt.Errorf("%w", result)
		}
	}

	return err
}

func (a app) SetStatusIfChanged(ctx context.Context, deviceID string, message Status) error {

	s := database.Status{
		DeviceID:     deviceID,
		BatteryLevel: message.BatteryLevel,
		Status:       message.Code,
		Messages:     strings.Join(message.Messages, ","),
		Timestamp:    message.Timestamp,
	}

	return a.db.SetStatusIfChanged(s)
}

func (a *app) sendMessage(event string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}

	message := sse.NewMessage("", string(b), event)
	a.sse.SendMessage("", message)

	return nil
}
