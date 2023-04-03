package events

import "time"

type BatteryLevelChanged struct {
	DeviceID   string    `json:"deviceID"`
	Tenant     string    `json:"tenant"`
	ObservedAt time.Time `json:"observedAt"`
}

func (b *BatteryLevelChanged) ContentType() string {
	return "application/json"
}
func (b *BatteryLevelChanged) TopicName() string {
	return "watchdog.batteryLevelChanged"
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
