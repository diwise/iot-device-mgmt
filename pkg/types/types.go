package types

import "time"

type Device struct {
	DevEUI       string       `json:"devEUI"`
	DeviceID     string       `json:"deviceID"`
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Location     Location     `json:"location"`
	Environment  string       `json:"environment"`
	Types        []string     `json:"types"`
	SensorType   SensorType   `json:"sensorType"`
	LastObserved time.Time    `json:"lastObserved"`
	Active       bool         `json:"active"`
	Tenant       string       `json:"tenant"`
	Status       DeviceStatus `json:"status"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Altitude  float64 `json:"altitude"`
}

type Environment struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type SensorType struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Interval    int    `json:"interval"`
}

type DeviceStatus struct {
	DeviceID     string   `json:"deviceID,omitempty"`
	BatteryLevel int      `json:"batteryLevel"`
	Code         int      `json:"statusCode"`
	Messages     []string `json:"statusMessages,omitempty"`
	Timestamp    string   `json:"timestamp"`
}

const StatusUnknown = -1
const StatusOK = 0
const StatusWarning = 1
const StatusError = 2
