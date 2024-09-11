package storage

import (
	"context"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestAddDevice(t *testing.T) {
	ctx, s := testSetup(t)

	err := s.AddDevice(ctx, newDevice())
	if err != nil {
		t.Fail()
	}
}

func TestGetDeviceByDeviceID(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	device := newDevice()
	err := s.AddDevice(ctx, device)
	is.NoErr(err)

	d, err := s.GetDevice(ctx, WithDeviceID(device.DeviceID))
	is.NoErr(err)
	is.Equal(d.DeviceID, device.DeviceID)
	is.Equal(d.Location.Latitude, device.Location.Latitude)
}

func TestGetDeviceByDeviceIDAndTenant(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	device := newDevice()
	err := s.AddDevice(ctx, device)
	is.NoErr(err)

	d, err := s.GetDevice(ctx, WithDeviceID(device.DeviceID), WithTenant(device.Tenant))
	is.NoErr(err)
	is.Equal(d.DeviceID, device.DeviceID)
	is.Equal(d.Location.Latitude, device.Location.Latitude)
}

func TestQueryDevice(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	device := newDevice()
	err := s.AddDevice(ctx, device)
	is.NoErr(err)

	result, err := s.QueryDevices(ctx, WithDeviceID(device.DeviceID), WithLimit(1))
	is.NoErr(err)
	is.Equal(len(result.Data), 1)
	is.Equal(result.TotalCount, uint64(1))
	is.Equal(result.Count, uint64(1))
}

func TestQueryDeviceOnline(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	device := newDevice()
	err := s.AddDevice(ctx, device)
	is.NoErr(err)

	result, err := s.QueryDevices(ctx, WithDeviceID(device.DeviceID), WithLimit(1), WithOnline(true))
	is.NoErr(err)
	is.Equal(len(result.Data), 1)
	is.Equal(result.TotalCount, uint64(1))
	is.Equal(result.Count, uint64(1))
}

func TestQueryDeviceTypes(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	suffix := uuid.NewString()

	device1 := newDevice()
	device1.DeviceProfile.Decoder = "decoder1" + suffix
	device2 := newDevice()
	device2.DeviceProfile.Decoder = "decoder2" + suffix
	err := s.AddDevice(ctx, device1)
	is.NoErr(err)
	err = s.AddDevice(ctx, device2)
	is.NoErr(err)

	result, err := s.QueryDevices(ctx, WithTypes([]string{"decoder1" + suffix, "decoder2" + suffix}), WithLimit(5), WithOnline(true))
	is.NoErr(err)
	is.Equal(len(result.Data), 2)
	is.Equal(result.TotalCount, uint64(2))
	is.Equal(result.Count, uint64(2))
}

func TestQueryDeviceBound(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)

	device := newDevice()
	err := s.AddDevice(ctx, device)
	is.NoErr(err)

	result, err := s.QueryDevices(ctx, WithBounds(61.0, 16.0, 63.0, 18.0))
	is.NoErr(err)
	is.Equal(len(result.Data), 1)
	is.Equal(result.TotalCount, uint64(1))
	is.Equal(result.Count, uint64(1))
}

func newDevice() types.Device {
	deviceID := uuid.NewString()
	sensorID := uuid.NewString()

	device := types.Device{
		Active:      true,
		SensorID:    sensorID,
		DeviceID:    deviceID,
		Tenant:      "default",
		Name:        "device1",
		Description: "device1",
		Location: types.Location{
			Latitude:  62.0,
			Longitude: 17.0,
		},
		Environment: "indoor",
		Source:      "source",
		Lwm2mTypes: []types.Lwm2mType{
			{
				Urn:  "urn:oma:lwm2m:ext:3311",
				Name: "3311",
			},
		},
		Tags: []types.Tag{
			{
				Name: "tag1",
			},
		},
		DeviceProfile: types.DeviceProfile{
			Name:     "profile1",
			Decoder:  "decoder1",
			Interval: 60,
			Types:    []string{"urn:oma:lwm2m:ext:3311"},
		},
		DeviceStatus: types.DeviceStatus{
			BatteryLevel: 100,
			ObservedAt:   time.Now(),
		},
		DeviceState: types.DeviceState{
			State:      types.DeviceStateOK,
			Online:     true,
			ObservedAt: time.Now(),
		},
	}
	return device
}

func testSetup(t *testing.T) (context.Context, *Storage) {
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
	err = s.CreateTables(ctx)
	if err != nil {
		t.SkipNow()
	}
	return ctx, s
}
