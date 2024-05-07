package types

import (
	"time"
)

type Device struct {
	Active      bool     `json:"active"`
	SensorID    string   `json:"sensorID"`
	DeviceID    string   `json:"deviceID"`
	Tenant      string   `json:"tenant"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Location    Location `json:"location"`
	Environment string   `json:"environment"`
	Source      string   `json:"source"`

	Lwm2mTypes []Lwm2mType `json:"types"`
	Tags       []Tag       `json:"tags"`

	DeviceProfile DeviceProfile `json:"deviceProfile"`

	DeviceStatus DeviceStatus `json:"deviceStatus"`
	DeviceState  DeviceState  `json:"deviceState"`

	Alarms []string `json:"alarms"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Tag struct {
	Name string `json:"name"`
}

type DeviceProfile struct {
	Name     string `json:"name"`
	Decoder  string `json:"decoder"`
	Interval int    `json:"interval"`
}

type Lwm2mType struct {
	Urn string `json:"urn"`
}

type DeviceStatus struct {
	BatteryLevel int       `json:"batteryLevel"`
	ObservedAt   time.Time `json:"observedAt"`
}

const (
	DeviceStateUnknown = -1
	DeviceStateOK      = 1
	DeviceStateWarning = 2
	DeviceStateError   = 3
)

type DeviceState struct {
	Online     bool      `json:"online"`
	State      int       `json:"state"`
	ObservedAt time.Time `json:"observedAt"`
}

const (
	AlarmSeverityLow    = 1
	AlarmSeverityMedium = 2
	AlarmSeverityHigh   = 3
)

type Alarm struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	AlarmType   string    `json:"alarmType"`
	Description string    `json:"description"`
	ObservedAt  time.Time `json:"observedAt"`
	RefID       string    `json:"refID"`
	Severity    int       `json:"severity"`
	Tenant      string    `json:"tenant"`
}
