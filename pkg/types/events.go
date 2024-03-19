package types

import (
	"encoding/json"
	"time"
)

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
func (d DeviceCreated) Body() []byte {
	b,_:=json.Marshal(d)
	return b
}

type DeviceUpdated struct {
	DeviceID  string    `json:"deviceID"`
	Tenant    string    `json:"tenant,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

func (d DeviceUpdated) ContentType() string {
	return "application/json"
}
func (d DeviceUpdated) TopicName() string {
	return "device.updated"
}

func (d DeviceUpdated) Body() []byte {
	b,_:=json.Marshal(d)
	return b
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
func (d DeviceStatusUpdated) Body() []byte {
	b,_:=json.Marshal(d)
	return b
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
func (d DeviceStateUpdated) Body() []byte {
	b,_:=json.Marshal(d)
	return b
}