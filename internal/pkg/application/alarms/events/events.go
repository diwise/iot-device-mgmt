package alarms

import "time"

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
