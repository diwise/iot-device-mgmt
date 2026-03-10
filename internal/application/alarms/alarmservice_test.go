package alarms

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/matryer/is"
)

func TestDeviceStatusHandler(t *testing.T) {
	is := is.New(t)
	log := slog.Default()
	ctx := context.Background()

	s := &AlarmStorageMock{
		AddFunc: func(ctx context.Context, deviceID string, a types.AlarmDetails) error {
			return nil
		},
	}
	m := &messaging.MsgContextMock{}

	svc := New(s, m, &Config{
		AlarmTypes: []types.AlarmType{
			{
				Name:    AlarmDeviceNotObserved,
				Enabled: true,
			},
		},
	})

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

	handler := newDeviceStatusHandler(svc)
	handler(ctx, msg, log)

	is.Equal(1, len(s.AddCalls()))
	is.Equal(AlarmDeviceNotObserved, s.AddCalls()[0].A.AlarmType)
}

func TestDeviceStatusHandlerWithMessages(t *testing.T) {
	is := is.New(t)
	log := slog.Default()
	ctx := context.Background()

	s := &AlarmStorageMock{
		AddFunc: func(ctx context.Context, deviceID string, a types.AlarmDetails) error {
			return nil
		},
	}
	m := &messaging.MsgContextMock{}

	svc := New(s, m, &Config{
		AlarmTypes: []types.AlarmType{
			{
				Name:    "message1",
				Enabled: true,
			},
			{
				Name:    "message2",
				Enabled: true,
			},
		},
	})

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

	handler := newDeviceStatusHandler(svc)
	handler(ctx, msg, log)

	is.Equal(2, len(s.AddCalls()))
	is.Equal("message2", s.AddCalls()[1].A.AlarmType)
}

func TestAlarmsQuery(t *testing.T) {
	is := is.New(t)
	ctx := context.Background()

	storage := &AlarmStorageMock{
		AlarmsFunc: func(ctx context.Context, query alarmquery.Alarms) (types.Collection[types.Alarms], error) {
			return types.Collection[types.Alarms]{
				Data: []types.Alarms{{DeviceID: "device-1"}},
			}, nil
		},
	}
	messenger := &messaging.MsgContextMock{}

	svc := New(storage, messenger, &Config{})
	_, err := svc.Alarms(ctx, alarmquery.Alarms{
		AllowedTenants: []string{"tenant-a"},
		AlarmType:      "battery_low",
	})

	is.NoErr(err)
	is.Equal(1, len(storage.AlarmsCalls()))
	is.True(storage.AlarmsCalls()[0].Query.ActiveOnly)
	is.Equal("battery_low", storage.AlarmsCalls()[0].Query.AlarmType)
	is.Equal([]string{"tenant-a"}, storage.AlarmsCalls()[0].Query.AllowedTenants)
}
