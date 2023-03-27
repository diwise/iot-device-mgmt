package types

import (
	"time"
)

type Device struct {
	Active      bool     `json:"active"`
	SensorID    string   `json:"sensorID"`
	DeviceID    string   `json:"deviceID"`
	Tenant      Tenant   `json:"tenant"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Location    Location `json:"location"`
	Environment string   `json:"environment"`

	Lwm2mTypes []Lwm2mType `json:"types"`
	Tags       []Tag       `json:"tags"`

	DeviceProfile DeviceProfile `json:"deviceProfile"`

	DeviceStatus DeviceStatus `json:"deviceStatus"`
	DeviceState  DeviceState  `json:"deviceState"`
	Alarms       []Alarm      `json:"alarms"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

type Tenant struct {
	Name string `json:"name"`
}

type Tag struct {
	Name string `json:"name"`
}

type DeviceProfile struct {
	Name    string `json:"name"`
	Decoder string `json:"decoder"`
}

type Lwm2mType struct {
	Urn string `json:"urn"`
}

type DeviceStatus struct {
	DeviceID     string    `json:"deviceID,omitempty"`
	BatteryLevel int       `json:"batteryLevel"`
	LastObserved time.Time `json:"lastObservedAt"`
}

const (
	DeviceStateUnknown = -1
	DeviceStateOffline = 0
	DeviceStateOK      = 1
	DeviceStateWarning = 2
	DeviceStateError   = 3
)

type DeviceState struct {
	State      int       `json:"state"`
	ObservedAt time.Time `json:"observedAt"`
}

const (
	AlarmSeverityLow    = 1
	AlarmSeverityMedium = 2
	AlarmSeverityHigh   = 3
)

type Alarm struct {
	ID          uint      `json:"id"`
	Type        string    `json:"type"`
	Severity    int       `json:"severity"`
	Description string    `json:"description"`
	Active      bool      `json:"active"`
	ObservedAt  time.Time `json:"observedAt"`
}

type DeviceStatistics struct {
	Total   int
	Online  int
	Offline int
	Warning int
	Error   int
	Alarms  int
}
