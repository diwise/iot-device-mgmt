package storage

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/matryer/is"
)

func testSetup(t *testing.T) (context.Context, Store) {
	ctx := context.Background()

	config := Config{
		host:                "localhost",
		user:                "postgres",
		password:            "password",
		port:                "5432",
		dbname:              "postgres",
		sslmode:             "disable",
		seedExistingDevices: true,
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
	is.True(!slices.ContainsFunc(c.Data, func(d types.Device) bool {
		return d.Active == false
	}))

	c, err = s.Query(ctx, WithActive(false))
	is.NoErr(err)

	is.True(!slices.ContainsFunc(c.Data, func(d types.Device) bool {
		return d.Active == true
	}))
}

func TestQueryWithSearch(t *testing.T) {
	is := is.New(t)
	ctx, s := testSetup(t)
	c, err := s.Query(ctx, WithSearch("70t589"))
	is.NoErr(err)
	is.True(len(c.Data) > 0)
}

/*
	func TestGetStaleDevices(t *testing.T) {
		is := is.New(t)
		ctx, s := testSetup(t)
		c, err := s.GetStaleDevices(ctx)
		is.NoErr(err)
		is.True(len(c.Data) > 0)
	}
*/
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

func setupTests(t *testing.T) (context.Context, *is.I, io.ReadCloser) {
	is := is.New(t)
	ctx := context.Background()

	h := &recordingHandler{}
	logger := slog.New(h)

	ctx = logging.NewContextWithLogger(ctx, logger)

	csv := io.NopCloser(strings.NewReader(`
sensor_id;device_id;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source;metadata
70t589;intern-70t589;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-70t589;desc-70t589;true;default;3600;source;key1=value1,key2=value2`))

	return ctx, is, csv
}

func TestSeedDevices_WithMetadata(t *testing.T) {
	ctx, is, csv := setupTests(t)

	s := &StoreMock{
		IsSeedExistingDevicesEnabledFunc: func(ctx context.Context) bool {
			return true
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, error) {
			return types.Device{}, ErrNoRows
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
	}

	err := SeedDevices(ctx, s, csv, []string{"default"})
	is.NoErr(err)
	is.Equal(1, len(s.CreateOrUpdateDeviceCalls()))
	is.Equal("intern-70t589", s.CreateOrUpdateDeviceCalls()[0].D.DeviceID)
	is.Equal("key1", s.CreateOrUpdateDeviceCalls()[0].D.Metadata[0].Key)
	is.Equal("value2", s.CreateOrUpdateDeviceCalls()[0].D.Metadata[1].Value)

	log := logging.GetFromContext(ctx)
	h, ok := log.Handler().(*recordingHandler)
	is.True(ok)
	is.Equal("seeded new device", h.records[1].Message)
}

func TestSeedDevices_NewDevice_And_ShouldNotSeedExistingDevices(t *testing.T) {
	ctx, is, csv := setupTests(t)

	s := &StoreMock{
		IsSeedExistingDevicesEnabledFunc: func(ctx context.Context) bool {
			return false
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, error) {
			return types.Device{}, ErrNoRows
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
	}

	err := SeedDevices(ctx, s, csv, []string{"default"})
	is.NoErr(err)
	is.Equal(1, len(s.CreateOrUpdateDeviceCalls()))
	is.Equal("intern-70t589", s.CreateOrUpdateDeviceCalls()[0].D.DeviceID)

	log := logging.GetFromContext(ctx)
	h, ok := log.Handler().(*recordingHandler)
	is.True(ok)
	is.Equal("seeded new device", h.records[1].Message)
}

func TestSeedDevices_NewDevice_And_ShouldSeedExistingDevices(t *testing.T) {
	ctx, is, csv := setupTests(t)
	s := &StoreMock{
		IsSeedExistingDevicesEnabledFunc: func(ctx context.Context) bool {
			return true
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, error) {
			return types.Device{}, ErrNoRows
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
	}

	err := SeedDevices(ctx, s, csv, []string{"default"})
	is.NoErr(err)
	is.Equal(1, len(s.CreateOrUpdateDeviceCalls()))
	is.Equal("intern-70t589", s.CreateOrUpdateDeviceCalls()[0].D.DeviceID)

	log := logging.GetFromContext(ctx)
	h, ok := log.Handler().(*recordingHandler)
	is.True(ok)
	is.Equal("seeded new device", h.records[1].Message)
}

func TestSeedDevices_ExistingDevice_And_ShouldSeedExistingDevices(t *testing.T) {
	ctx, is, csv := setupTests(t)
	s := &StoreMock{
		IsSeedExistingDevicesEnabledFunc: func(ctx context.Context) bool {
			return true
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, error) {
			return types.Device{}, nil
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
	}

	err := SeedDevices(ctx, s, csv, []string{"default"})
	is.NoErr(err)
	is.Equal(1, len(s.CreateOrUpdateDeviceCalls()))

	log := logging.GetFromContext(ctx)
	h, ok := log.Handler().(*recordingHandler)
	is.True(ok)
	is.Equal("updated existing device", h.records[1].Message)
}

func TestSeedDevices_ExistingDevice_And_ShouldNotSeedExistingDevices(t *testing.T) {
	ctx, is, csv := setupTests(t)
	s := &StoreMock{
		IsSeedExistingDevicesEnabledFunc: func(ctx context.Context) bool {
			return false
		},
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, error) {
			return types.Device{}, nil
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
	}

	err := SeedDevices(ctx, s, csv, []string{"default"})
	is.NoErr(err)
	is.Equal(0, len(s.CreateOrUpdateDeviceCalls()))

	log := logging.GetFromContext(ctx)
	h, ok := log.Handler().(*recordingHandler)
	is.True(ok)
	is.Equal("seed should not update existing devices", h.records[1].Message)
}

const devices_csv string = `
devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source;metadata
70t589;intern-70t589;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-70t589;desc-70t589;true;default;3600;;key1=value1
50t555;intern-70t555;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-70t555;desc-70t555;true;default;3600;;
30t333;intern-70t333;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-70t333;desc-70t333;true;default;3600;;`

type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	r2 := slog.Record{
		Time:    r.Time,
		Level:   r.Level,
		PC:      r.PC,
		Message: r.Message,
	}
	r.Attrs(func(a slog.Attr) bool { r2.AddAttrs(a); return true })

	h.mu.Lock()
	h.records = append(h.records, r2)
	h.mu.Unlock()
	return nil
}

func (h *recordingHandler) WithAttrs(as []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler         { return h }
