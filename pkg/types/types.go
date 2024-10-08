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
	Environment string   `json:"environment,omitempty"`
	Source      string   `json:"source,omitempty"`

	Lwm2mTypes []Lwm2mType `json:"types"`
	Tags       []Tag       `json:"tags,omitempty"`

	DeviceProfile DeviceProfile `json:"deviceProfile"`

	DeviceStatus DeviceStatus `json:"deviceStatus"`
	DeviceState  DeviceState  `json:"deviceState"`

	Alarms []string `json:"alarms,omitempty"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Tag struct {
	Name string `json:"name"`
}

type DeviceProfile struct {
	Name     string   `json:"name" yaml:"name"`
	Decoder  string   `json:"decoder" yaml:"decoder"`
	Interval int      `json:"interval" yaml:"interval"`
	Types    []string `json:"types,omitempty" yaml:"types"`
}

type Lwm2mType struct {
	Urn  string `json:"urn" yaml:"urn"`
	Name string `json:"name" yaml:"name"`
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
	AlarmType   string    `json:"alarmType"`
	Description string    `json:"description,omitempty"`
	ObservedAt  time.Time `json:"observedAt"`
	RefID       string    `json:"refID"`
	Severity    int       `json:"severity"`
	Tenant      string    `json:"tenant"`
}

type InformationItem struct {
	DeviceID   string    `json:"deviceID"`
	ObservedAt time.Time `json:"observedAt"`
	Types      []string  `json:"types"`
}

type Collection[T any] struct {
	Data       []T
	Count      uint64
	Offset     uint64
	Limit      uint64
	TotalCount uint64
}

type Bounds struct {
	MinLon float64
	MaxLon float64
	MinLat float64
	MaxLat float64
}
