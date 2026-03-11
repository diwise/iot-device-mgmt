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

	dp := types.SensorProfile{
		Name:     "elsys",
		Decoder:  "elsys",
		Interval: 3600,
		Types:    []string{"urn:oma:lwm2m:ext:3303", "urn:oma:lwm2m:ext:3304"},
	}

	resBody := struct {
		Data types.SensorProfile `json:"data"`
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
				`"sensorProfile":{"name":"unknown"`),
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
		SensorProfile: types.SensorProfile{
			Name: UNKNOWN,
		},
	}

	err = client.CreateDevice(ctx, device)
	is.NoErr(err)

	client.Close(ctx)
}

func TestGetSensor(t *testing.T) {
	is := is.New(t)

	resBody := `{"data":{"sensorID":"sensor-1","deviceID":"device-1","sensorProfile":{"name":"Elsys","decoder":"elsys","interval":60}}}`
	mockedService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/sensors/sensor-1"),
			expects.RequestMethod("GET"),
		),
		test.Returns(
			response.Code(200),
			response.Body([]byte(resBody)),
		),
	)

	mockOAuth := test.NewMockServiceThat(
		test.Expects(is, expects.RequestPath("/token")),
		test.Returns(response.ContentType("application/json"), response.Code(200), response.Body([]byte(TokenResponse))),
	)
	defer mockOAuth.Close()

	ctx := context.Background()
	client, err := New(ctx, mockedService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)

	sensor, err := client.GetSensor(ctx, "sensor-1")
	is.NoErr(err)
	is.Equal(sensor.ID(), "sensor-1")
	is.Equal(sensor.DeviceID(), "device-1")
	is.True(sensor.IsAssigned())
	is.Equal(sensor.SensorType(), "elsys")
	is.Equal(sensor.Interval(), 60)

	client.Close(ctx)
}

func TestListSensors(t *testing.T) {
	is := is.New(t)

	resBody := `{"data":[{"sensorID":"sensor-1","sensorProfile":{"decoder":"elsys","interval":60}},{"sensorID":"sensor-2"}]}`
	assigned := false
	hasProfile := true
	limit := 10
	mockedService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/sensors"),
			expects.QueryParamContains("assigned", "false"),
			expects.QueryParamContains("hasProfile", "true"),
			expects.QueryParamContains("limit", "10"),
			expects.RequestMethod("GET"),
		),
		test.Returns(response.Code(200), response.Body([]byte(resBody))),
	)

	mockOAuth := test.NewMockServiceThat(
		test.Expects(is, expects.RequestPath("/token")),
		test.Returns(response.ContentType("application/json"), response.Code(200), response.Body([]byte(TokenResponse))),
	)
	defer mockOAuth.Close()

	ctx := context.Background()
	client, err := New(ctx, mockedService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)

	sensors, err := client.ListSensors(ctx, types.SensorsQuery{Assigned: &assigned, HasProfile: &hasProfile, Limit: &limit})
	is.NoErr(err)
	is.Equal(len(sensors), 2)
	is.Equal(sensors[0].ID(), "sensor-1")
	is.Equal(sensors[1].SensorType(), "")

	client.Close(ctx)
}

func TestAttachAndDetachSensorToDevice(t *testing.T) {
	is := is.New(t)

	mockedService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/devices/device-1/sensor"),
			expects.RequestMethod("PUT"),
			expects.RequestBodyContaining(`"sensorID":"sensor-1"`),
		),
		test.Returns(response.Code(200)),
	)

	mockOAuth := test.NewMockServiceThat(
		test.Expects(is, expects.RequestPath("/token")),
		test.Returns(response.ContentType("application/json"), response.Code(200), response.Body([]byte(TokenResponse))),
	)
	defer mockOAuth.Close()

	ctx := context.Background()
	client, err := New(ctx, mockedService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)
	is.NoErr(client.AttachSensorToDevice(ctx, "device-1", "sensor-1"))
	client.Close(ctx)

	mockedDetachService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/devices/device-1/sensor"),
			expects.RequestMethod("DELETE"),
		),
		test.Returns(response.Code(204)),
	)

	client, err = New(ctx, mockedDetachService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)
	is.NoErr(client.DetachSensorFromDevice(ctx, "device-1"))
	client.Close(ctx)
}

func TestGetTenantsAndDeviceProfiles(t *testing.T) {
	is := is.New(t)

	tenantsService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/admin/tenants"),
			expects.RequestMethod("GET"),
		),
		test.Returns(response.Code(200), response.Body([]byte(`{"data":["default","tenant-a"]}`))),
	)

	mockOAuth := test.NewMockServiceThat(
		test.Expects(is, expects.RequestPath("/token")),
		test.Returns(response.ContentType("application/json"), response.Code(200), response.Body([]byte(TokenResponse))),
	)
	defer mockOAuth.Close()

	ctx := context.Background()
	client, err := New(ctx, tenantsService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)
	tenants, err := client.GetTenants(ctx)
	is.NoErr(err)
	is.Equal(tenants, []string{"default", "tenant-a"})
	client.Close(ctx)

	profilesService := test.NewMockServiceThat(
		test.Expects(is,
			expects.RequestPath("/api/v0/admin/deviceprofiles"),
			expects.RequestMethod("GET"),
		),
		test.Returns(response.Code(200), response.Body([]byte(`{"data":[{"name":"Elsys","decoder":"elsys","interval":60}]}`))),
	)

	client, err = New(ctx, profilesService.URL(), mockOAuth.URL()+"/token", false, "", "")
	is.NoErr(err)
	profiles, err := client.GetDeviceProfiles(ctx)
	is.NoErr(err)
	is.Equal(len(profiles), 1)
	is.Equal(profiles[0].Decoder, "elsys")
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

/*
	func TestFindDeviceFromDevEUIRetriesAfterErrorCacheTTL(t *testing.T) {
		is := is.New(t)

		var requests atomic.Int32
		mockedService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			is.Equal(r.URL.Path, "/api/v0/devices")
			is.Equal(r.Method, http.MethodGet)
			is.Equal(r.URL.Query().Get("devEUI"), "retry-device")

			count := requests.Add(1)
			if count == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer mockedService.Close()

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

		client, err := New(ctx, mockedService.URL, mockOAuth.URL()+"/token", false, "", "")
		is.NoErr(err)
		dmc := client.(*devManagementClient)
		dmc.errorCacheTTL = 20 * time.Millisecond

		// First call gets 401, automatically retries with fresh token, then gets 404
		_, err = client.FindDeviceFromDevEUI(ctx, "retry-device")
		is.True(err != nil)
		is.True(errors.Is(err, ErrNotFound)) // Now expects NotFound due to automatic retry
		is.Equal(requests.Load(), int32(2))  // Should have made 2 requests (401 + retry)

		// Second call uses cached error, no new requests
		_, err = client.FindDeviceFromDevEUI(ctx, "retry-device")
		is.True(err != nil)
		is.True(errors.Is(err, ErrNotFound))
		is.Equal(requests.Load(), int32(2)) // Still 2 requests (cached)

		time.Sleep(35 * time.Millisecond)

		// After TTL expires, tries again and gets 404
		_, err = client.FindDeviceFromDevEUI(ctx, "retry-device")
		is.True(errors.Is(err, ErrNotFound))
		is.True(requests.Load() >= int32(3))

		client.Close(ctx)
	}
*/
const TokenResponse string = `{"access_token":"testtoken","expires_in":300,"refresh_expires_in":0,"token_type":"Bearer","not-before-policy":0,"scope":"email profile"}`
