package types

import "time"

type Device struct {
	DevEUI       string       `json:"devEUI"`
	DeviceId     string       `json:"deviceID"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Location     Location     `json:"location"`
	Environment  string       `json:"environment"`
	Types        []string     `json:"types"`
	SensorType   string       `json:"sensor_type"`
	LastObserved time.Time    `json:"last_observed"`
	Active       bool         `json:"active"`
	Tenant       string       `json:"tenant"`
	Status       DeviceStatus `json:"status"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitue   float64 `json:"altitude"`
}

type DeviceStatus struct {
	BatteryLevel int      `json:"batteryLevel"`
	Code         int      `json:"statusCode"`
	Messages     []string `json:"statusMessages,omitempty"`
	Timestamp    string   `json:"timestamp"`
}
