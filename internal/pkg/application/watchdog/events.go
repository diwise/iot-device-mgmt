package watchdog

import (
	"encoding/json"
	"time"
)

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
	b, _ := json.Marshal(l)
	return b
}
