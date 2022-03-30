package application

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

type Device interface {
}

type DeviceManagement interface {
	GetDevice(context.Context, string) (Device, error)
}

func New(logger zerolog.Logger) DeviceManagement {
	a := &app{
		devices: map[string]Device{},
		logger:  logger,
	}

	a.devices["a81758fffe06bfa3"] = device{ID: "intern-a81758fffe06bfa3", Types: []string{"urn:oma:lwm2m:ext:3303"}}

	return a
}

type app struct {
	devices map[string]Device
	logger  zerolog.Logger
}

func (a *app) GetDevice(ctx context.Context, externalID string) (Device, error) {

	device, ok := a.devices[externalID]
	if !ok {
		return nil, fmt.Errorf("no such device")
	}

	return device, nil
}

type device struct {
	ID    string   `json:"id"`
	Types []string `json:"types"`
}
