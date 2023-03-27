package events

import "time"

type BatteryLevelWarning struct {
	DeviceID   string    `json:"deviceID"`
	ObservedAt time.Time `json:"observedAt"`
}

func (b *BatteryLevelWarning) ContentType() string {
	return "application/json"
}
func (b *BatteryLevelWarning) TopicName() string {
	return "alarms.batteryLevelWarning"
}

type LastObservedWarning struct {
	DeviceID   string    `json:"deviceID"`
	ObservedAt time.Time `json:"observedAt"`
}

func (l *LastObservedWarning) ContentType() string {
	return "application/json"
}
func (l *LastObservedWarning) TopicName() string {
	return "alarms.lastObservedWarning"
}
