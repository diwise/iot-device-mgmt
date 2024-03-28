package events

import (
	"encoding/json"
	"time"
)

type BatteryLevelChanged struct {
	DeviceID     string    `json:"deviceID"`
	BatteryLevel int       `json:"batteryLevel"`
	Tenant       string    `json:"tenant"`
	ObservedAt   time.Time `json:"observedAt"`
}

func (b BatteryLevelChanged) ContentType() string {
	return "application/json"
}
func (b BatteryLevelChanged) TopicName() string {
	return "watchdog.batteryLevelChanged"
}
func (l BatteryLevelChanged) Body() []byte {
	b,_:=json.Marshal(l)
	return b
}
type DeviceNotObserved struct {
	DeviceID   string    `json:"deviceID"`
	Tenant     string    `json:"tenant"`
	ObservedAt time.Time `json:"observedAt"`
}

func (l *DeviceNotObserved) ContentType() string {
	return "application/json"
}
func (l *DeviceNotObserved) TopicName() string {
	return "watchdog.deviceNotObserved"
}
func (l *DeviceNotObserved) Body() []byte {
	b,_:=json.Marshal(l)
	return b
}