package alarms

import (
	"encoding/json"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

const AlarmDeviceNotObserved string = "DeviceNotObserved"

type AlarmCreated struct {
	Alarm     types.Alarm `json:"alarm"`
	Tenant    string      `json:"tenant"`
	Timestamp time.Time   `json:"timestamp"`
}

func (l *AlarmCreated) ContentType() string {
	return "application/json"
}
func (l *AlarmCreated) TopicName() string {
	return "alarms.alarmCreated"
}
func (l *AlarmCreated) Body() []byte {
	b, _ := json.Marshal(l)
	return b
}

type AlarmClosed struct {
	ID        string    `json:"id"`
	Tenant    string    `json:"tenant"`
	Timestamp time.Time `json:"timestamp"`
}

func (l *AlarmClosed) ContentType() string {
	return "application/json"
}
func (l *AlarmClosed) TopicName() string {
	return "alarms.alarmClosed"
}
func (l *AlarmClosed) Body() []byte {
	b, _ := json.Marshal(l)
	return b
}
