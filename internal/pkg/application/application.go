package application

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
)

type Device interface {
	ID() string
}

type DeviceManagement interface {
	GetDevice(context.Context, string) (Device, error)
}

func New(logger zerolog.Logger) DeviceManagement {
	a := &app{
		devices: map[string]Device{},
		logger:  logger,
	}

	a.devices["a81758fffe06bfa3"] = device{Identity: "intern-a81758fffe06bfa3", Types: []string{"urn:oma:lwm2m:ext:3303"}}

	return a
}

type app struct {
	devices map[string]Device
	logger  zerolog.Logger
}

func (a *app) GetDevice(ctx context.Context, externalID string) (Device, error) {

	device, ok := a.devices[externalID]
	if !ok {
		return nil, fmt.Errorf("no such device (%s)", externalID)
	}

	return device, nil
}

type device struct {
	Identity string   `json:"id"`
	Types    []string `json:"types"`
}

func (d device) ID() string {
	return d.Identity
}
