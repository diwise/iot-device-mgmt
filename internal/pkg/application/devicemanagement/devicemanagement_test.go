package devicemanagement

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/matryer/is"
)

func TestNewSeeds(t *testing.T) {
	ctx, is, s := testSetup(t)
	var err error

	err = s.CreateTables(ctx)
	is.NoErr(err)

	err = SeedLwm2mTypes(ctx, s, []types.Lwm2mType{
		{
			Urn:  "urn:diwise:1",
			Name: "name",
		},
	})
	is.NoErr(err)

	err = SeedDeviceProfiles(ctx, s, []types.DeviceProfile{
		{
			Name:     "elsys",
			Decoder:  "elsys",
			Interval: 60,
			Types: []string{
				"urn:diwise:1",
			},
		},
	})
	is.NoErr(err)

	err = SeedDevices(ctx, s, io.NopCloser(strings.NewReader(devices_test)), []string{"default"})
	is.NoErr(err)
}

func testSetup(t *testing.T) (context.Context, *is.I, *storage.Storage) {
	ctx := context.Background()
	is := is.New(t)

	s, err := storage.New(ctx, storage.NewConfig("localhost", "postgres", "password", "5432", "postgres", "disable"))
	if err != nil {
		t.SkipNow()
	}
	err = s.CreateTables(ctx)
	if err != nil {
		t.SkipNow()
	}
	return ctx, is, s
}

const devices_test string = `
devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
70t589;intern-70t589;62.39160;17.30723;water;urn:diwise:1;elsys;name-70t589;desc-70t589;true;default;3600;
`
