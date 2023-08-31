package client

import (
	"context"
	"testing"

	test "github.com/diwise/service-chassis/pkg/test/http"
	"github.com/diwise/service-chassis/pkg/test/http/expects"
	"github.com/diwise/service-chassis/pkg/test/http/response"
	"github.com/matryer/is"
)

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
	defer c.Close(ctx)

	is.NoErr(err)
}

func TestGetDeviceByDevEUI(t *testing.T) {
	is := is.New(t)

	s := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/devices"),
		),
		test.Returns(
			response.ContentType("application/json"),
			response.Code(200),
			response.Body([]byte(DeviceByDevEUIResponse)),
		),
	)

	dmc := &devManagementClient{
		url: s.URL(),
	}

	_, err := dmc.findDeviceFromDevEUI(context.Background(), "test")
	is.NoErr(err)
}

const DeviceByDevEUIResponse string = `{"meta":{"totalRecords":1,"count":1},"data":[{"active":true,"sensorID":"a81758fffe06bfa3","deviceID":"intern-a81758fffe06bfa3","tenant":{"name":"default"},"name":"name-a81758fffe06bfa3","description":"desc-a81758fffe06bfa3","location":{"latitude":62.3916,"longitude":17.30723,"altitude":0},"environment":"water","source":"source","types":[{"urn":"urn:oma:lwm2m:ext:3303"},{"urn":"urn:oma:lwm2m:ext:3302"},{"urn":"urn:oma:lwm2m:ext:3301"}],"tags":[],"deviceProfile":{"name":"elsys_codec","decoder":"elsys_codec","interval":60},"deviceStatus":{"batteryLevel":-1,"lastObservedAt":"0001-01-01T00:00:00Z"},"deviceState":{"online":false,"state":-1,"observedAt":"0001-01-01T00:00:00Z"}}],"links":{"self":"https://diwise.io/api/v0/devices?devEUI=a81758fffe06bfa3"}}`
const TokenResponse string = `{"access_token":"testtoken","expires_in":300,"refresh_expires_in":0,"token_type":"Bearer","not-before-policy":0,"scope":"email profile"}`
