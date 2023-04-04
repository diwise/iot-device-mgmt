package alarms

import (
	"time"

	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
)

type AlarmCreated struct {
	Alarm     db.Alarm  `json:"alarm"`
	Timestamp time.Time `json:"timestamp"`
}

func (l *AlarmCreated) ContentType() string {
	return "application/json"
}
func (l *AlarmCreated) TopicName() string {
	return "alarms.alarmCreated"
}

type AlarmClosed struct {
	ID        int       `json:"id"`
	Timestamp time.Time `json:"timestamp"`
}

func (l *AlarmClosed) ContentType() string {
	return "application/json"
}
func (l *AlarmClosed) TopicName() string {
	return "alarms.alarmClosed"
}
