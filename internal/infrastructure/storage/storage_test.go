package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
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

	t.Run("create sensor for device", func(t *testing.T) {
		err := s.CreateSensor(ctx, types.Sensor{
			SensorID: sensorID,
			SensorProfile: &types.SensorProfile{
				Decoder: "testdecoder",
			},
		})
		if err != nil {
			t.Fatalf("failed to create sensor for device: %v", err)
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

	t.Run("create standalone sensor", func(t *testing.T) {
		name := "Standalone test sensor"
		err := s.CreateSensor(ctx, types.Sensor{
			SensorID: "zz-test-sensor-" + uuid.NewString(),
			Name:     &name,
			Location: &types.Location{Latitude: 62.3901, Longitude: 17.3069},
			SensorProfile: &types.SensorProfile{
				Decoder: "testdecoder-2",
			},
		})
		if err != nil {
			t.Fatalf("failed to create sensor: %v", err)
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
		d, found, err := s.GetDeviceBySensorID(ctx, sensorID)
		if err != nil {
			t.Fatalf("failed to get device by sensor ID: %v", err)
		}
		if !found {
			t.Fatal("expected device to be found")
		}
		if d.DeviceID != deviceID {
			t.Fatalf("expected device ID '%s', got '%s'", deviceID, d.DeviceID)
		}
	})

	t.Run("get missing device by sensor ID", func(t *testing.T) {
		_, found, err := s.GetDeviceBySensorID(ctx, "missing-sensor-id")
		if err != nil {
			t.Fatalf("expected no error for missing device, got %v", err)
		}
		if found {
			t.Fatal("expected missing device to return found=false")
		}
	})

	standaloneSensorID := "zz-test-sensor-standalone-" + uuid.NewString()

	t.Run("create and get standalone sensor", func(t *testing.T) {
		name := "Standalone roundtrip sensor"
		err := s.CreateSensor(ctx, types.Sensor{
			SensorID: standaloneSensorID,
			Name:     &name,
			Location: &types.Location{Latitude: 63.101, Longitude: 17.802},
			SensorProfile: &types.SensorProfile{
				Decoder: "testdecoder-2",
			},
		})
		if err != nil {
			t.Fatalf("failed to create standalone sensor: %v", err)
		}

		sensor, found, err := s.GetSensor(ctx, standaloneSensorID)
		if err != nil {
			t.Fatalf("failed to get sensor: %v", err)
		}
		if !found {
			t.Fatal("expected sensor to be found")
		}
		if sensor.SensorID != standaloneSensorID {
			t.Fatalf("expected sensor id %q, got %q", standaloneSensorID, sensor.SensorID)
		}
		if sensor.Name == nil || *sensor.Name != name {
			t.Fatalf("expected sensor name %q, got %+v", name, sensor.Name)
		}
		if sensor.Location == nil || sensor.Location.Latitude != 63.101 || sensor.Location.Longitude != 17.802 {
			t.Fatalf("expected sensor location {63.101 17.802}, got %+v", sensor.Location)
		}
		if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder != "TestDecoder-2" {
			t.Fatalf("expected joined sensor profile decoder TestDecoder-2, got %+v", sensor.SensorProfile)
		}

		_, found, err = s.GetDeviceBySensorID(ctx, standaloneSensorID)
		if err != nil {
			t.Fatalf("expected no error when checking device by sensor id, got %v", err)
		}
		if found {
			t.Fatal("expected standalone sensor to not create a device")
		}
	})

	t.Run("query sensors", func(t *testing.T) {
		limit := 1000
		result, err := s.QuerySensors(ctx, sensorquery.Sensors{Limit: &limit})
		if err != nil {
			t.Fatalf("failed to query sensors: %v", err)
		}
		if result.Count == 0 {
			t.Fatal("expected at least one sensor in query result")
		}

		foundStandalone := false
		for _, sensor := range result.Data {
			if sensor.SensorID == standaloneSensorID {
				foundStandalone = true
				break
			}
		}
		if !foundStandalone {
			t.Fatalf("expected query result to contain sensor %q", standaloneSensorID)
		}
	})

	t.Run("query unassigned sensors with profile", func(t *testing.T) {
		limit := 1000
		assigned := false
		hasProfile := true
		result, err := s.QuerySensors(ctx, sensorquery.Sensors{Limit: &limit, Assigned: &assigned, HasProfile: &hasProfile})
		if err != nil {
			t.Fatalf("failed to query sensors with filters: %v", err)
		}

		for _, sensor := range result.Data {
			if sensor.DeviceID != nil {
				t.Fatalf("expected unassigned sensors only, got %+v", sensor)
			}
			if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder == "" {
				t.Fatalf("expected sensors with profile only, got %+v", sensor)
			}
		}
	})

	t.Run("query sensors by profileName case insensitive", func(t *testing.T) {
		limit := 1000
		result, err := s.QuerySensors(ctx, sensorquery.Sensors{Limit: &limit, ProfileName: "TESTDECODER-2"})
		if err != nil {
			t.Fatalf("failed to query sensors by profileName: %v", err)
		}
		if result.Count == 0 {
			t.Fatal("expected at least one sensor for profileName filter")
		}

		for _, sensor := range result.Data {
			if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder != "TestDecoder-2" {
				t.Fatalf("expected only TestDecoder-2 sensors, got %+v", sensor.SensorProfile)
			}
		}
	})

	t.Run("query sensors by profile type", func(t *testing.T) {
		limit := 1000
		result, err := s.QuerySensors(ctx, sensorquery.Sensors{Limit: &limit, Types: []string{"URN:OMA:LWM2M:EXT:3303"}})
		if err != nil {
			t.Fatalf("failed to query sensors by profile type: %v", err)
		}
		if result.Count == 0 {
			t.Fatal("expected at least one sensor for profile type filter")
		}

		foundStandalone := false
		for _, sensor := range result.Data {
			if sensor.SensorProfile == nil {
				t.Fatalf("expected profile on typed sensor, got %+v", sensor)
			}
			if sensor.SensorID == standaloneSensorID {
				foundStandalone = true
			}
		}
		if !foundStandalone {
			t.Fatalf("expected typed sensor query to include sensor %q", standaloneSensorID)
		}
	})

	t.Run("update standalone sensor", func(t *testing.T) {
		name := "Updated standalone sensor"
		err := s.UpdateSensor(ctx, types.Sensor{
			SensorID: standaloneSensorID,
			Name:     &name,
			Location: &types.Location{Latitude: 64.001, Longitude: 18.123},
			SensorProfile: &types.SensorProfile{
				Decoder: "testdecoder",
			},
		})
		if err != nil {
			t.Fatalf("failed to update sensor: %v", err)
		}

		sensor, found, err := s.GetSensor(ctx, standaloneSensorID)
		if err != nil {
			t.Fatalf("failed to get updated sensor: %v", err)
		}
		if !found {
			t.Fatal("expected updated sensor to be found")
		}
		if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder != "TestDecoder" {
			t.Fatalf("expected updated decoder TestDecoder, got %+v", sensor.SensorProfile)
		}
		if sensor.Name == nil || *sensor.Name != name {
			t.Fatalf("expected updated name %q, got %+v", name, sensor.Name)
		}
		if sensor.Location == nil || sensor.Location.Latitude != 64.001 || sensor.Location.Longitude != 18.123 {
			t.Fatalf("expected updated location {64.001 18.123}, got %+v", sensor.Location)
		}
	})

	t.Run("query", func(t *testing.T) {
		devices, err := s.Query(ctx, dmquery.Devices{Filters: dmquery.Filters{SensorID: sensorID}})
		if err != nil {
			t.Fatalf("failed to query devices: %v", err)
		}
		if len(devices.Data) != 1 {
			t.Fatalf("expected 1 device, got %d", len(devices.Data))
		}
	})

	t.Run("query alarms", func(t *testing.T) {
		err := s.Add(ctx, deviceID, types.AlarmDetails{
			AlarmType:   "battery_low",
			Description: "Battery is low",
			ObservedAt:  time.Now().UTC(),
			Severity:    2,
		})
		if err != nil {
			t.Fatalf("failed to add alarm: %v", err)
		}

		result, err := s.Alarms(ctx, alarmquery.Alarms{
			AllowedTenants: []string{"test-tenant"},
			ActiveOnly:     true,
		})
		if err != nil {
			t.Fatalf("failed to query alarms: %v", err)
		}
		found := false
		for _, alarm := range result.Data {
			if alarm.DeviceID == deviceID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected alarm results to contain device id %q, got %+v", deviceID, result.Data)
		}
	})

	t.Run("add device status for missing device", func(t *testing.T) {
		err := s.AddDeviceStatus(ctx, types.StatusMessage{
			DeviceID:  "missing-device-id",
			Timestamp: time.Now().UTC(),
		})
		if !errors.Is(err, ErrStatusDeviceNotFound) {
			t.Fatalf("expected ErrStatusDeviceNotFound, got %v", err)
		}
	})

	t.Run("set active flag on device", func(t *testing.T) {
		d, found, err := s.GetDeviceBySensorID(ctx, sensorID)
		if err != nil {
			t.Fatalf("failed to get device by sensor ID: %v", err)
		}
		if !found {
			t.Fatal("expected device to be found")
		}

		activeStatus := d.Active
		newStatus := !activeStatus
		err = s.UpdateDevice(ctx, deviceID, &newStatus, nil, nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("failed to set active flag: %v", err)
		}

		d, found, err = s.GetDeviceBySensorID(ctx, sensorID)
		if err != nil {
			t.Fatalf("failed to get device by sensor ID: %v", err)
		}
		if !found {
			t.Fatal("expected device to be found")
		}

		if d.Active != newStatus {
			t.Fatalf("expected active flag to be %v, got %v", newStatus, d.Active)
		}
	})

	t.Run("attach standalone sensor to device", func(t *testing.T) {
		err := s.AssignSensor(ctx, deviceID, standaloneSensorID)
		if err != nil {
			t.Fatalf("failed to assign sensor: %v", err)
		}

		d, found, err := s.GetDeviceBySensorID(ctx, standaloneSensorID)
		if err != nil {
			t.Fatalf("failed to get device by newly assigned sensor: %v", err)
		}
		if !found || d.DeviceID != deviceID {
			t.Fatalf("expected assigned sensor to point to device %q, got %+v found=%v", deviceID, d, found)
		}
	})

	t.Run("detach sensor from device", func(t *testing.T) {
		err := s.UnassignSensor(ctx, deviceID)
		if err != nil {
			t.Fatalf("failed to unassign sensor: %v", err)
		}

		_, found, err := s.GetDeviceBySensorID(ctx, standaloneSensorID)
		if err != nil {
			t.Fatalf("failed to verify detached sensor: %v", err)
		}
		if found {
			t.Fatal("expected detached sensor to no longer be linked to a device")
		}
	})

}
