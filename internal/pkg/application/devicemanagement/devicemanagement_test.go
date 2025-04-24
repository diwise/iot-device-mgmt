package devicemanagement

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestDeviceStatusHandler(t *testing.T) {
	is := is.New(t)
	log := slog.Default()
	ctx := context.Background()

	storage := &DeviceStorageMock{
		QueryFunc: func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
			return types.Collection[types.Device]{}, nil
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
		AddDeviceStatusFunc: func(ctx context.Context, status types.StatusMessage) error {
			return nil
		},
	}

	msgCtx := messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
	}

	bat := 99.0

	sm := types.StatusMessage{
		DeviceID:     uuid.NewString(),
		Tenant:       "default",
		Timestamp:    time.Now(),
		BatteryLevel: &bat,
	}

	svc := New(storage, &msgCtx, nil)

	err := svc.NewDevice(ctx, types.Device{
		Active:      true,
		SensorID:    uuid.NewString(),
		DeviceID:    sm.DeviceID,
		Tenant:      "default",
		Name:        "Test",
		Description: "Test",
		Location: types.Location{
			Latitude:  0,
			Longitude: 0,
		},
		Environment: "",
		Source:      "",
		Lwm2mTypes: []types.Lwm2mType{
			{
				Urn:  "urn:xxx:1",
				Name: "1",
			},
		},
		Tags: []types.Tag{},
		DeviceProfile: types.DeviceProfile{
			Name:     "test",
			Decoder:  "test",
			Interval: 0,
			Types:    []string{"urn:xxx:1"},
		},
		DeviceStatus: types.DeviceStatus{},
		DeviceState:  types.DeviceState{},
		Alarms:       []string{},
	})
	is.NoErr(err)

	handler := NewDeviceStatusHandler(svc)
	handler(ctx, statusMessage(sm), log)
}

func statusMessage(s types.StatusMessage) messaging.IncomingTopicMessage {
	return &messaging.IncomingTopicMessageMock{
		BodyFunc: func() []byte {
			b, _ := json.Marshal(s)
			return b
		},
	}
}
