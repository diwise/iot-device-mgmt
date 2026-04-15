package devices

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestDeviceStatusHandler(t *testing.T) {
	is := is.New(t)
	log := slog.Default()
	ctx := context.Background()

	reader := &DeviceReaderMock{
		QueryFunc: func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
			return types.Collection[types.Device]{}, nil
		},
		GetSensorFunc: func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
			return types.Sensor{
				SensorID: sensorID,
				SensorProfile: &types.SensorProfile{
					Decoder: "test",
				},
			}, true, nil
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, bool, error) {
			return types.Device{}, false, nil
		},
	}
	writer := &DeviceWriterMock{
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
		SetDeviceProfileTypesFunc: func(ctx context.Context, deviceID string, typesMoqParam []types.Lwm2mType) error {
			return nil
		},
	}
	statusWriter := &DeviceStatusWriterMock{
		AddDeviceStatusFunc: func(ctx context.Context, status types.StatusMessage) error {
			return nil
		},
		SetDeviceStateFunc: func(ctx context.Context, deviceID string, state types.DeviceState) error {
			return nil
		},
	}
	profiles := &DeviceProfileStoreMock{}

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

	svc := New(reader, writer, statusWriter, profiles, &msgCtx, nil)

	err := svc.Create(ctx, types.Device{
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
		SensorProfile: types.SensorProfile{
			Name:     "test",
			Decoder:  "test",
			Interval: 0,
			Types:    []string{"urn:xxx:1"},
		},
		SensorStatus: types.SensorStatus{},
		DeviceState:  types.DeviceState{},
		Alarms:       []string{},
	})
	is.NoErr(err)

	handler := newDeviceStatusHandler(svc)
	handler(ctx, statusMessage(sm), log)
}

func TestCreateRequiresSensorProfileForAssignedSensor(t *testing.T) {
	is := is.New(t)

	reader := &DeviceReaderMock{
		QueryFunc: func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
			return types.Collection[types.Device]{}, nil
		},
		GetSensorFunc: func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
			return types.Sensor{SensorID: sensorID}, true, nil
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, bool, error) {
			return types.Device{}, false, nil
		},
	}

	svc := New(reader, &DeviceWriterMock{}, &DeviceStatusWriterMock{}, &DeviceProfileStoreMock{}, &messaging.MsgContextMock{}, nil)
	err := svc.Create(context.Background(), types.Device{DeviceID: "device-1", SensorID: "sensor-1", Tenant: "default"})
	is.True(errors.Is(err, ErrSensorProfileRequired))
}

func TestAttachSensorRejectsAssignedSensor(t *testing.T) {
	is := is.New(t)

	reader := &DeviceReaderMock{
		QueryFunc: func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
			return types.Collection[types.Device]{Count: 1, Data: []types.Device{{DeviceID: "device-1", Tenant: "default"}}}, nil
		},
		GetSensorFunc: func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
			return types.Sensor{SensorID: sensorID, SensorProfile: &types.SensorProfile{Decoder: "elsys"}}, true, nil
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, bool, error) {
			return types.Device{DeviceID: "device-2", Tenant: "default"}, true, nil
		},
	}

	svc := New(reader, &DeviceWriterMock{}, &DeviceStatusWriterMock{}, &DeviceProfileStoreMock{}, &messaging.MsgContextMock{}, nil)
	err := svc.AttachSensor(context.Background(), "device-1", "sensor-1", []string{"default"})
	is.True(errors.Is(err, ErrSensorAlreadyAssigned))
}

func TestDetachSensorCallsWriter(t *testing.T) {
	is := is.New(t)
	called := false

	reader := &DeviceReaderMock{
		QueryFunc: func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
			return types.Collection[types.Device]{Count: 1, Data: []types.Device{{DeviceID: "device-1", Tenant: "default", SensorID: "sensor-1"}}}, nil
		},
	}
	writer := &DeviceWriterMock{
		UnassignSensorFunc: func(ctx context.Context, deviceID string) error {
			called = true
			is.Equal(deviceID, "device-1")
			return nil
		},
	}

	svc := New(reader, writer, &DeviceStatusWriterMock{}, &DeviceProfileStoreMock{}, &messaging.MsgContextMock{}, nil)
	err := svc.DetachSensor(context.Background(), "device-1", []string{"default"})
	is.NoErr(err)
	is.True(called)
}

func statusMessage(s types.StatusMessage) messaging.IncomingTopicMessage {
	return &messaging.IncomingTopicMessageMock{
		BodyFunc: func() []byte {
			b, _ := json.Marshal(s)
			return b
		},
	}
}
