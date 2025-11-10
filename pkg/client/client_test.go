package client

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	test "github.com/diwise/service-chassis/pkg/test/http"
	"github.com/diwise/service-chassis/pkg/test/http/expects"
	"github.com/diwise/service-chassis/pkg/test/http/response"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

const UNKNOWN = "unknown"

func TestGetProfile(t *testing.T) {
	is := is.New(t)

	dp := types.DeviceProfile{
		Name:     "elsys",
		Decoder:  "elsys",
		Interval: 3600,
		Types:    []string{"urn:oma:lwm2m:ext:3303", "urn:oma:lwm2m:ext:3304"},
	}

	resBody := struct {
		Data types.DeviceProfile `json:"data"`
	}{
		Data: dp,
	}

	b, _ := json.Marshal(resBody)

	mockedService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/admin/deviceprofiles"),
			expects.QueryParamContains("name", "elsys"),
			expects.RequestMethod("GET"),
		),
		test.Returns(
			response.Code(200),
			response.Body(b),
		),
	)

	mockOAuth := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/token"),
		),
		test.Returns(
			response.ContentType("application/json"),
			response.Code(200),
			response.Body([]byte(TokenResponse)),
		),
	)
	defer mockOAuth.Close()

	ctx := context.Background()

	client, err := New(ctx, mockedService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)

	profile, err := client.GetDeviceProfile(ctx, "elsys")
	is.NoErr(err)

	is.Equal(profile.Name, "elsys")
	is.Equal(profile.Decoder, "elsys")
	is.Equal(profile.Interval, 3600)
	is.Equal(profile.Types, []string{"urn:oma:lwm2m:ext:3303", "urn:oma:lwm2m:ext:3304"})

	client.Close(ctx)
}

func TestCreateUnknownDevice(t *testing.T) {
	is := is.New(t)

	mockedService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/devices"),
			expects.RequestMethod("POST"),
			expects.RequestHeaderContains("Content-Type", "application/json"),
			expects.RequestBodyContaining(`"active":false`,
				`"sensorID":"testsensorid"`,
				`"deviceProfile":{"name":"unknown"`),
		),
		test.Returns(
			response.Code(201),
		),
	)

	mockOAuth := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/token"),
		),
		test.Returns(
			response.ContentType("application/json"),
			response.Code(200),
			response.Body([]byte(TokenResponse)),
		),
	)
	defer mockOAuth.Close()

	ctx := context.Background()

	client, err := New(ctx, mockedService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)

	device := types.Device{
		Active:   false,
		DeviceID: uuid.New().String(),
		SensorID: "testsensorid",
		Name:     "testdevice",
		DeviceProfile: types.DeviceProfile{
			Name: UNKNOWN,
		},
	}

	err = client.CreateDevice(ctx, device)
	is.NoErr(err)

	client.Close(ctx)
}

func TestMe(t *testing.T) {
	is := is.New(t)

	s := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/token"),
		),
		test.Returns(
			response.ContentType("application/json"),
			response.Code(200),
			response.Body([]byte(TokenResponse)),
		),
	)

	ctx := context.Background()

	c, err := New(ctx, s.URL(), s.URL()+"/token", false, "", "")
	is.NoErr(err)

	c.Close(ctx)
}

const TokenResponse string = `{"access_token":"testtoken","expires_in":300,"refresh_expires_in":0,"token_type":"Bearer","not-before-policy":0,"scope":"email profile"}`
