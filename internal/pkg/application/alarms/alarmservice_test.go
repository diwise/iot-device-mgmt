package alarms

import (
	"context"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestAddAlarm(t *testing.T) {
	is, ctx, s, m := testSetup(t)
	service := New(s, &m)

	alarm := newAlarm()
	err := service.Add(ctx, alarm)
	is.NoErr(err)
}

func TestCloseAlarm(t *testing.T) {
	is, ctx, s, m := testSetup(t)
	service := New(s, &m)

	alarm := newAlarm()
	err := service.Add(ctx, alarm)
	is.NoErr(err)

	err = service.Close(ctx, alarm.ID, []string{alarm.Tenant})
	is.NoErr(err)

	// close already closed alarm
	err = service.Close(ctx, alarm.ID, []string{alarm.Tenant})
	is.NoErr(err)

	// 1 create and 1 closed
	is.Equal(2, len(m.PublishOnTopicCalls()))
}

func newAlarm() types.Alarm {
	alarm := types.Alarm{
		ID:          uuid.NewString(),
		AlarmType:   "alarm1",
		Description: "alarm1",
		ObservedAt:  time.Now(),
		RefID:       uuid.NewString(),
		Severity:    1,
		Tenant:      "default",
	}
	return alarm
}

func testSetup(t *testing.T) (*is.I, context.Context, *storage.Storage, messaging.MsgContextMock) {
	ctx := context.Background()
	is := is.New(t)

	config := storage.NewConfig(
		"localhost",
		"postgres",
		"password",
		"5432",
		"postgres",
		"disable",
	)
	s, err := storage.New(ctx, config)
	if err != nil {
		t.SkipNow()
	}
	err = s.CreateTables(ctx)
	if err != nil {
		t.SkipNow()
	}
	m := messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
		PublishOnTopicFunc: func(ctx context.Context, message messaging.TopicMessage) error {
			return nil
		},
	}

	return is, ctx, s, m
}
