package alarms

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/matryer/is"
)

func TestDeviceStatusHandler(t *testing.T) {
	is := is.New(t)
	log := slog.Default()
	ctx := context.Background()

	s := &AlarmStorageMock{
		AddAlarmFunc: func(ctx context.Context, deviceID string, a types.Alarm) error {
			return nil
		},
	}
	m := &messaging.MsgContextMock{}

	svc := New(s, m)

	msg := &messaging.IncomingTopicMessageMock{
		BodyFunc: func() []byte {
			code := AlarmDeviceNotObserved
			observedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
			status := types.StatusMessage{
				Code:      &code,
				Timestamp: observedAt,
			}
			b, _ := json.Marshal(status)
			return b
		},
	}

	handler := NewDeviceStatusHandler(svc)
	handler(ctx, msg, log)

	is.Equal(1, len(s.AddAlarmCalls()))
	is.Equal(AlarmDeviceNotObserved, s.AddAlarmCalls()[0].A.AlarmType)
}

func TestDeviceStatusHandlerWithMessages(t *testing.T) {
	is := is.New(t)
	log := slog.Default()
	ctx := context.Background()

	s := &AlarmStorageMock{
		AddAlarmFunc: func(ctx context.Context, deviceID string, a types.Alarm) error {
			return nil
		},
	}
	m := &messaging.MsgContextMock{}

	svc := New(s, m)

	msg := &messaging.IncomingTopicMessageMock{
		BodyFunc: func() []byte {
			code := ""
			observedAt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
			status := types.StatusMessage{
				Code:      &code,
				Timestamp: observedAt,
				Messages:  []string{"message1", "message2"},
			}
			b, _ := json.Marshal(status)
			return b
		},
	}

	handler := NewDeviceStatusHandler(svc)
	handler(ctx, msg, log)

	is.Equal(2, len(s.AddAlarmCalls()))
	is.Equal("message2", s.AddAlarmCalls()[1].A.AlarmType)
}
