package devicemanagement

import (
	"context"
	"testing"
	"time"

	repository "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/devicemanagement"
	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
)

func TestUpdateDevice(t *testing.T) {
	ctx, repo, msgCtx := testSetup(t)
	svc := New(repo, msgCtx)

	deviceID := uuid.NewString()
	err := svc.CreateDevice(ctx, types.Device{
		Active:      true,
		SensorID:    uuid.NewString(),
		DeviceID:    deviceID,
		Tenant:      "default",
		Name:        "test device",
		Description: "",
		Location: types.Location{
			Latitude:  0.0,
			Longitude: 0.0,
		},
		Environment: "",
		Source:      "",
		Lwm2mTypes: []types.Lwm2mType{
			{Urn: "urn:3"},
		},
		Tags: []types.Tag{},
		DeviceProfile: types.DeviceProfile{
			Name:     "test",
			Decoder:  "test",
			Interval: 3600,
		},
		DeviceStatus: types.DeviceStatus{
			BatteryLevel: 100,
			ObservedAt:   time.Now(),
		},
		DeviceState: types.DeviceState{
			Online:     true,
			State:      0,
			ObservedAt: time.Now(),
		},
	})

	if err != nil {
		t.Log("could not create device")
		t.FailNow()
	}

	device, err := svc.GetDeviceByDeviceID(ctx, deviceID, []string{"default"})
	if err != nil {
		t.FailNow()
	}

	if device.DeviceID != deviceID {
		t.FailNow()
	}

	err = svc.UpdateDevice(ctx, deviceID, map[string]any{
		"name":        "changed",
		"description": "changed",
	}, []string{"default"})

	if err != nil {
		t.FailNow()
	}

	device, err = svc.GetDeviceByDeviceID(ctx, deviceID, []string{"default"})
	if err != nil {
		t.FailNow()
	}

	if device.Name != "changed" || device.Description != "changed" {
		t.Log("properties not updated")
		t.FailNow()
	}
}

func testSetup(t *testing.T) (context.Context, repository.DeviceRepository, messaging.MsgContext) {
	ctx := context.Background()
	p, err := jsonstore.NewPool(ctx, jsonstore.NewConfig(
		"localhost",
		"postgres",
		"password",
		"5432",
		"postgres",
		"disable",
	))
	if err != nil {
		t.SkipNow()
	}

	repo, err := repository.NewRepository(ctx, p)
	if err != nil {
		t.SkipNow()
	}

	msgCtx := &messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
		PublishOnTopicFunc: func(ctx context.Context, message messaging.TopicMessage) error {
			return nil
		},
	}

	return ctx, repo, msgCtx
}
