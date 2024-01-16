package client

import (
	"context"
	"testing"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	test "github.com/diwise/service-chassis/pkg/test/http"
	"github.com/diwise/service-chassis/pkg/test/http/expects"
	"github.com/diwise/service-chassis/pkg/test/http/response"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

const UNKNOWN = "unknown"

func TestCreateUnknownDevice(t *testing.T) {
	is := is.New(t)

	mockedService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/devices/"),
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

	client, err := New(ctx, mockedService.URL(), mockOAuth.URL()+"/token", "", "")
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

	c, err := New(ctx, s.URL(), s.URL()+"/token", "", "")
	is.NoErr(err)

	c.Close(ctx)
}

const TokenResponse string = `{"access_token":"testtoken","expires_in":300,"refresh_expires_in":0,"token_type":"Bearer","not-before-policy":0,"scope":"email profile"}`
