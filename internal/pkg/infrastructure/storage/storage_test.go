package storage

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/matryer/is"
)

func testSetup(t *testing.T) (context.Context, Store) {
	ctx := context.Background()

	config := Config{
		host:     "localhost",
		user:     "postgres",
		password: "password",
		port:     "5432",
		dbname:   "postgres",
		sslmode:  "disable",
	}

	s, err := New(ctx, config)
	if err != nil {
		t.SkipNow()
	}

	err = s.Initialize(ctx)
	if err != nil {
		t.SkipNow()
	}

	err = SeedLwm2mTypes(ctx, s, []types.Lwm2mType{
		{
			Urn:  "urn:oma:lwm2m:ext:3303",
			Name: "Temperature",
		},
		{
			Urn:  "urn:oma:lwm2m:ext:3302",
			Name: "Humidity",
		},
		{
			Urn:  "urn:oma:lwm2m:ext:3301",
			Name: "Illuminance",
		},
	})
	if err != nil {
		t.SkipNow()
	}

	err = SeedDeviceProfiles(ctx, s, []types.DeviceProfile{
		{
			Name:     "Elsys_Codec",
			Decoder:  "Elsys_Codec",
			Interval: 3600,
			Types: []string{
				"urn:oma:lwm2m:ext:3301",
				"urn:oma:lwm2m:ext:3302",
				"urn:oma:lwm2m:ext:3303",
			},
		},
	})
	if err != nil {
		t.SkipNow()
	}

	err = SeedDevices(ctx, s, io.NopCloser(strings.NewReader(devices_csv)), []string{"default"})
	if err != nil {
		t.SkipNow()
	}

	return ctx, s
}

func TestQuery(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.Query(ctx)
	is.NoErr(err)
	is.True(len(c.Data) > 0)
}

func TestSetDevice(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.Query(ctx)
	is.NoErr(err)
	is.True(len(c.Data) > 0)

	d := c.Data[0]
	env := "air"
	err = s.SetDevice(ctx, d.DeviceID, nil, nil, nil, &env, nil, nil, nil, nil)
	is.NoErr(err)
}

func TestQueryWithDeviceID(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.Query(ctx, WithDeviceID("intern-70t589"))
	is.NoErr(err)
	is.True(len(c.Data) > 0)
}

func TestQueryWithSensorID(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.Query(ctx, WithSensorID("70t589"))
	is.NoErr(err)
	is.True(len(c.Data) > 0)
}

func TestQueryWithSensorIDAndDeviceID(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.Query(ctx, WithDeviceID("intern-70t589"), WithSensorID("70t589"))
	is.NoErr(err)
	is.True(len(c.Data) > 0)
}

func TestQueryWithActive(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	c, err := s.Query(ctx, WithActive(true))
	is.NoErr(err)
	is.True(len(c.Data) > 0)

	c, err = s.Query(ctx, WithActive(false))
	is.NoErr(err)
	is.True(len(c.Data) == 0)
}

func TestQueryWithSearch(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.Query(ctx, WithSearch("70t589"))
	is.NoErr(err)
	is.True(len(c.Data) > 0)
}

func TestGetStaleDevices(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.GetStaleDevices(ctx)
	is.NoErr(err)
	is.True(len(c.Data) > 0)
}

func TestGetSensorByID(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	d, err := s.GetDeviceBySensorID(ctx, "70t589")
	is.NoErr(err)
	is.Equal("intern-70t589", d.DeviceID)
	is.Equal(3, len(d.Lwm2mTypes))
}

func TestGetDeviceStatus(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	_, err := s.GetDeviceStatus(ctx, "intern-70t589")
	is.NoErr(err)
}

func setupSeedDevices(t *testing.T, updateExistingDevices string, trackedID map[string]bool) (createCalls int, updateCalls int) {
	t.Helper()
	mock := &StoreMock{
		GetUpdateExistingDevicesFunc: func(ctx context.Context) string {
			return updateExistingDevices
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, error) {
			if exists, ok := trackedID[sensorID]; ok && exists {
				return types.Device{SensorID: sensorID}, nil
			}
			return types.Device{}, ErrNoRows
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			if exists, ok := trackedID[d.SensorID]; ok {
				if !exists {
					createCalls++
				} else {
					updateCalls++
				}
			}
			return nil
		},
	}
	err := SeedDevices(context.Background(), mock, io.NopCloser(strings.NewReader(devices_csv)), []string{"default"})
	if err != nil {
		t.Fatalf("SeedDevice returned unexpected error: %v", err)
	}
	return createCalls, updateCalls
}

func TestSeedDevices_CreatesWhenIdNotExist(t *testing.T) {
	trackingID := map[string]bool{
		"70t589": false,
	}
	createCalls, updateCalls := setupSeedDevices(t, "true", trackingID)
	if createCalls != 1 {
		t.Errorf("expected 1 CreateOrUpdateDevice calls, got %d", createCalls)
	}
	if updateCalls != 0 {
		t.Errorf("expected 0 updates, got %d", updateCalls)
	}
}

func TestSeedDevices_SkipUpdatesWhenIdExistsAndSkipUpdateTrue(t *testing.T) {
	trackingID := map[string]bool{
		"70t589": true,
	}
	createCalls, updateCalls := setupSeedDevices(t, "false", trackingID)
	if createCalls != 0 {
		t.Errorf("expected 0 CreateOrUpdateDevice calls, got %d", createCalls)
	}
	if updateCalls != 0 {
		t.Errorf("expected 0 updates, got %d", updateCalls)
	}
}

func TestSeedDevices_UpdateWhenIdExistAndSkipUpdateFalse(t *testing.T) {
	trackingID := map[string]bool{
		"70t589": true,
	}
	createCalls, updateCalls := setupSeedDevices(t, "true", trackingID)
	if createCalls != 0 {
		t.Errorf("expected 0 creates calls, got %d", createCalls)
	}
	if updateCalls != 1 {
		t.Errorf("expected 2 updates, got %d", updateCalls)
	}
}

const devices_csv string = `
devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
70t589;intern-70t589;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-70t589;desc-70t589;true;default;3600;`
