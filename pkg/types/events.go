package types

import "time"

type DeviceCreated struct {
	DeviceID  string    `json:"deviceID"`
	Tenant    string    `json:"tenant,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (d *DeviceCreated) ContentType() string {
	return "application/json"
}
func (d *DeviceCreated) TopicName() string {
	return "device.created"
}

type DeviceUpdated struct {
	DeviceID  string    `json:"deviceID"`
	Tenant    string    `json:"tenant,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (d *DeviceUpdated) ContentType() string {
	return "application/json"
}
func (d *DeviceUpdated) TopicName() string {
	return "device.updated"
}

type DeviceStatusUpdated struct {
	DeviceID     string       `json:"deviceID"`
	DeviceStatus DeviceStatus `json:"status"`
	Tenant       string       `json:"tenant,omitempty"`
	Timestamp    time.Time    `json:"timestamp"`
}

func (d *DeviceStatusUpdated) ContentType() string {
	return "application/json"
}
func (d *DeviceStatusUpdated) TopicName() string {
	return "device.statusUpdated"
}

type DeviceStateUpdated struct {
	DeviceID  string    `json:"deviceID"`
	State     int       `json:"state"`
	Tenant    string    `json:"tenant,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (d *DeviceStateUpdated) ContentType() string {
	return "application/json"
}
func (d *DeviceStateUpdated) TopicName() string {
	return "device.stateUpdated"
}

