package application

import (
	"context"
	"fmt"
)

type Device interface {
	ID() string
}

type DeviceManagement interface {
	GetDevice(context.Context, string) (Device, error)
}

func New() DeviceManagement {
	a := &app{
		devices: knownDevices,
	}

	return a
}

type app struct {
	devices map[string]Device
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

var knownDevices map[string]Device = map[string]Device{
	"a81758fffe06bfa3": device{Identity: "intern-a81758fffe06bfa3", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe051d00": device{Identity: "intern-a81758fffe051d00", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe04d83f": device{Identity: "intern-a81758fffe04d83f", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe0524f3": device{Identity: "intern-a81758fffe0524f3", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe04d84d": device{Identity: "intern-a81758fffe04d84d", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe04d843": device{Identity: "intern-a81758fffe04d843", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe04d851": device{Identity: "intern-a81758fffe04d851", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe051d02": device{Identity: "intern-a81758fffe051d02", Types: []string{"urn:oma:lwm2m:ext:3303"}},
	"a81758fffe04d856": device{Identity: "intern-a81758fffe04d856", Types: []string{"urn:oma:lwm2m:ext:3303"}},
}
