package alarms

import (
	"encoding/json"
	"time"

	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
)

type AlarmCreated struct {
	Alarm     db.Alarm  `json:"alarm"`
	Tenant    string    `json:"tenant"`
	Timestamp time.Time `json:"timestamp"`
}

func (l *AlarmCreated) ContentType() string {
	return "application/json"
}
func (l *AlarmCreated) TopicName() string {
	return "alarms.alarmCreated"
}
func (l *AlarmCreated) Body() []byte {
	b,_:=json.Marshal(l)
	return b
}

type AlarmClosed struct {
	ID        int       `json:"id"`
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
	b,_:=json.Marshal(l)
	return b
}
