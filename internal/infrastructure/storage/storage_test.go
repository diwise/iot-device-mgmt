package storage

import (
	"context"
	"testing"

	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/google/uuid"
)

func testSetup(t *testing.T) (context.Context, *Storage) {
	cfg := NewConfig("localhost", "postgres", "postgres", "5432", "postgres", "disable")
	s, err := New(context.Background(), cfg)
	if err != nil {
		t.Skip("could not connect to database")
	}

	ctx := context.Background()

	return ctx, s
}

func TestStorage(t *testing.T) {
	ctx, s := testSetup(t)
	defer s.Close()

	deviceID := "test-device-" + uuid.NewString()
	sensorID := "test-sensor-" + uuid.NewString()

	t.Run("create profile with new type", func(t *testing.T) {
		err := s.CreateSensorProfile(ctx, types.SensorProfile{
			Name:     "TestProfile",
			Decoder:  "TestDecoder",
			Interval: 60,
			Types:    []string{"urn:oma:lwm2m:ext:3303"},
		})
		if err != nil {
			t.Fatalf("failed to create sensor profile: %v", err)
		}

		err = s.CreateSensorProfile(ctx, types.SensorProfile{
			Name:     "TestProfile-2",
			Decoder:  "TestDecoder-2",
			Interval: 60,
			Types:    []string{"urn:oma:lwm2m:ext:3303"},
		})
		if err != nil {
			t.Fatalf("failed to create sensor profile: %v", err)
		}
	})

	t.Run("create sensor profile type", func(t *testing.T) {
		tType := types.Lwm2mType{
			Urn:  "urn:oma:lwm2m:ext:3303",
			Name: "Temperature",
		}

		err := s.CreateSensorProfileType(ctx, tType)
		if err != nil {
			t.Fatalf("failed to create sensor profile type: %v", err)
		}
	})

	t.Run("create a device", func(t *testing.T) {
		d := types.Device{
			DeviceID: deviceID,
			SensorID: sensorID,
			Active:   true,
			Tenant:   "test-tenant",
			SensorProfile: types.SensorProfile{
				Name:     "TestProfile",
				Decoder:  "TestDecoder",
				Interval: 60,
				Types:    []string{"urn:oma:lwm2m:ext:3303"},
			},
			Interval:    0,
			Name:        "Test Device 1",
			Description: "",
			Metadata:    []types.Metadata{},
			Location: types.Location{
				Latitude:  0,
				Longitude: 0,
			},
			Environment:  "",
			Source:       "",
			Lwm2mTypes:   []types.Lwm2mType{},
			Tags:         []types.Tag{},
			DeviceState:  types.DeviceState{},
			SensorStatus: types.SensorStatus{},
		}

		err := s.CreateOrUpdateDevice(ctx, d)
		if err != nil {
			t.Fatalf("failed to create device: %v", err)
		}
	})

	t.Run("set sensor profile", func(t *testing.T) {
		err := s.SetSensorProfile(ctx, deviceID, types.SensorProfile{
			Name:     "TestProfile-2",
			Decoder:  "TestDecoder-2",
			Interval: 60,
			Types:    []string{"urn:oma:lwm2m:ext:3303"},
		})
		if err != nil {
			t.Fatalf("failed to set sensor profile: %v", err)
		}
	})

	t.Run("set device profile types", func(t *testing.T) {
		err := s.SetDeviceProfileTypes(ctx, deviceID, []types.Lwm2mType{
			{
				Urn:  "urn:oma:lwm2m:ext:3304",
				Name: "Humidity",
			},
		})
		if err != nil {
			t.Fatalf("failed to set device profile types: %v", err)
		}
	})

	t.Run("update device", func(t *testing.T) {
		description := "Updated description"
		err := s.UpdateDevice(ctx, deviceID, nil, nil, &description, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("failed to update device: %v", err)
		}
	})

	t.Run("get device by sensor ID", func(t *testing.T) {
		d, err := s.GetDeviceBySensorID(ctx, sensorID)
		if err != nil {
			t.Fatalf("failed to get device by sensor ID: %v", err)
		}
		if d.DeviceID != deviceID {
			t.Fatalf("expected device ID '%s', got '%s'", deviceID, d.DeviceID)
		}
	})

	t.Run("query", func(t *testing.T) {
		devices, err := s.Query(ctx, conditions.WithSensorID(sensorID))
		if err != nil {
			t.Fatalf("failed to query devices: %v", err)
		}
		if len(devices.Data) != 1 {
			t.Fatalf("expected 1 device, got %d", len(devices.Data))
		}
	})

	t.Run("set active flag on device", func(t *testing.T) {
		d, err := s.GetDeviceBySensorID(ctx, sensorID)
		if err != nil {
			t.Fatalf("failed to get device by sensor ID: %v", err)
		}

		activeStatus := d.Active
		newStatus := !activeStatus
		err = s.UpdateDevice(ctx, deviceID, &newStatus, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("failed to set active flag: %v", err)
		}

		d, err = s.GetDeviceBySensorID(ctx, sensorID)
		if err != nil {
			t.Fatalf("failed to get device by sensor ID: %v", err)
		}

		if d.Active != newStatus {
			t.Fatalf("expected active flag to be %v, got %v", newStatus, d.Active)
		}
	})

}
