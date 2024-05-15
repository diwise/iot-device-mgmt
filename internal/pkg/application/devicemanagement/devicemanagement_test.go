package devicemanagement

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	repository "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/devicemanagement"
	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
	"github.com/matryer/is"
	"gopkg.in/yaml.v2"
)

func TestUpdateDevice(t *testing.T) {
	_, ctx, _, _, svc := testSetup(t)

	deviceID := uuid.NewString()
	d := newDevice(deviceID)
	err := svc.Create(ctx, d)

	if err != nil {
		t.Log("could not create device")
		t.FailNow()
	}

	device, err := svc.GetByDeviceID(ctx, deviceID, []string{"default"})
	if err != nil {
		t.FailNow()
	}

	if device.DeviceID != deviceID {
		t.FailNow()
	}

	err = svc.Merge(ctx, deviceID, map[string]any{
		"name":        "changed",
		"description": "changed",
	}, []string{"default"})

	if err != nil {
		t.FailNow()
	}

	device, err = svc.GetByDeviceID(ctx, deviceID, []string{"default"})
	if err != nil {
		t.FailNow()
	}

	if device.Name != "changed" || device.Description != "changed" {
		t.Log("properties not updated")
		t.FailNow()
	}
}

func TestAlarmCreatedHandler(t *testing.T) {
	is, ctx, _, _, svc := testSetup(t)

	handler := NewAlarmCreatedHandler(svc)

	deviceID := uuid.NewString()
	d := newDevice(deviceID)

	err := svc.Create(ctx, d)
	is.NoErr(err)

	alarmID := uuid.NewString()
	itm := newAlarmCreated(alarmID, deviceID)

	handler(ctx, itm, &slog.Logger{})

	d, err = svc.GetByDeviceID(ctx, deviceID, []string{"default"})
	is.NoErr(err)

	is.True(slices.Contains(d.Alarms, alarmID))
}

func TestAlarmClosedHandler(t *testing.T) {
	is, ctx, _, _, svc := testSetup(t)

	alarmCreatedHandler := NewAlarmCreatedHandler(svc)

	deviceID := uuid.NewString()
	d := newDevice(deviceID)
	is.Equal(0, d.DeviceState.State)

	err := svc.Create(ctx, d)
	is.NoErr(err)

	alarmID := uuid.NewString()

	log := &slog.Logger{}

	alarmCreatedHandler(ctx, newAlarmCreated(alarmID, deviceID), log)

	d, err = svc.GetByDeviceID(ctx, deviceID, []string{"default"})
	is.NoErr(err)

	is.Equal(2, d.DeviceState.State)
	is.True(slices.Contains(d.Alarms, alarmID))

	alarmClosedHandler := NewAlarmClosedHandler(svc)
	alarmClosedHandler(ctx, newAlarmClosed(alarmID), log)

	d, err = svc.GetByDeviceID(ctx, deviceID, []string{"default"})
	is.NoErr(err)

	is.Equal(1, d.DeviceState.State)
	is.True(!slices.Contains(d.Alarms, alarmID))
	is.Equal(0, len(d.Alarms))
}

func TestDeviceNotFound(t *testing.T) {
	is, ctx, _, _, svc := testSetup(t)
	_, err := svc.GetBySensorID(ctx, uuid.NewString(), []string{"default"})
	is.True(errors.Is(err, ErrDeviceNotFound))
}

func TestGetWithAlarmID(t *testing.T) {
	is, ctx, _, _, svc := testSetup(t)

	device, err := svc.GetBySensorID(ctx, "5679", []string{"default"})
	is.NoErr(err)

	is.Equal("intern-5679", device.DeviceID)

	alarmID := uuid.NewString()
	device.Alarms = append(device.Alarms, alarmID)
	err = svc.Create(ctx, device)
	is.NoErr(err)

	deviceWithAlarm, err := svc.GetWithAlarmID(ctx, alarmID, []string{"default"})
	is.NoErr(err)

	is.Equal(device.DeviceID, deviceWithAlarm.DeviceID)
}

func TestSeed(t *testing.T) {
	is, ctx, _, _, svc := testSetup(t)
	devices, err := svc.Get(ctx, 0, 100, "", []string{"default"})
	is.NoErr(err)
	is.True(devices.TotalCount > 0)
}

func TestDeviceProfiles(t *testing.T) {
	is := is.New(t)
	b := []byte(configYaml)
	dp := DeviceManagementConfig{}
	err := yaml.Unmarshal(b, &dp)
	is.NoErr(err)
}

func newAlarmClosed(alarmID string) *alarms.AlarmClosed {
	return &alarms.AlarmClosed{
		ID:        alarmID,
		Tenant:    "default",
		Timestamp: time.Now(),
	}
}

func newAlarmCreated(alarmID, deviceID string) *alarms.AlarmCreated {
	return &alarms.AlarmCreated{
		Alarm: types.Alarm{
			ID:          alarmID,
			Type:        "Alarm",
			AlarmType:   "test",
			Description: "",
			ObservedAt:  time.Now(),
			RefID:       deviceID,
			Severity:    1,
			Tenant:      "default",
		},
		Tenant:    "default",
		Timestamp: time.Now(),
	}
}

func newDevice(deviceID string) types.Device {
	return types.Device{
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
	}
}

func testSetup(t *testing.T) (*is.I, context.Context, repository.DeviceRepository, messaging.MsgContext, DeviceManagement) {
	is := is.New(t)
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

	cfg := &DeviceManagementConfig{}
	is.NoErr(yaml.Unmarshal([]byte(configYaml), cfg))

	svc := New(repo, msgCtx, cfg)

	r := bytes.NewBuffer([]byte(csvMock))
	svc.Seed(ctx, r, []string{"default"})

	return is, ctx, repo, msgCtx, svc
}

const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;_default;60;source
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;_default;60;source
1234;intern-1234;0.0;0.0;air;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3304;enviot;name-1234;desc-1234;true;_test;60;källa
5678;intern-5678;0.0;0.0;soil;urn:oma:lwm2m:ext:3303;enviot;name-5678;desc-5678;true;_test;60;
5679;intern-5679;0.0;0.0;;urn:oma:lwm2m:ext:3330,urn:oma:lwm2m:ext:3;axsensor;AXsensor;Mäter nivå i avlopp;true;default;0;
`

const configYaml string = `
deviceprofiles:
  - name: qalcosonic
    decoder: qalcosonic
    interval: 3600
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3424
      - urn:oma:lwm2m:ext:3303
  - name: axsensor
    decoder: axsensor
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3330
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3327
      - urn:oma:lwm2m:ext:3303
types:
  - urn : urn:oma:lwm2m:ext:3
    name: Device 
  - urn: urn:oma:lwm2m:ext:3303
    name: Temperature
`
