package types

import "time"

type DeviceCreated struct {
	DeviceID  string    `json:"deviceID"`
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
	Timestamp    time.Time    `json:"timestamp"`
}

func (d *DeviceStatusUpdated) ContentType() string {
	return "application/json"
}
func (d *DeviceStatusUpdated) TopicName() string {
	return "device.statusUpdated"
}
