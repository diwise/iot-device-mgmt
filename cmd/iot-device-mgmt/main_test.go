package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"gopkg.in/yaml.v2"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"

	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/go-chi/jwtauth/v5"
	"github.com/matryer/is"
)

const noToken string = ""

func TestThatHealthEndpointReturns204NoContent(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, _ := testRequest(server, http.MethodGet, "/health", noToken, nil)

	is.Equal(resp.StatusCode, http.StatusNoContent)
}

func TestThatGetUnknownDeviceReturns404(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"default"})
	resp, _ := testRequest(server, http.MethodGet, "/api/v0/devices/nosuchdevice", token, nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func TestThatGetKnownDeviceByEUIReturns200(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"default"})
	resp, body := testRequest(server, http.MethodGet, "/api/v0/devices?devEUI=a81758fffe06bfa3", token, nil)

	d := struct {
		Data struct {
			DevEui string `json:"sensorID"`
		} `json:"data"`
	}{}
	json.Unmarshal([]byte(body), &d)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal("a81758fffe06bfa3", d.Data.DevEui)
}

func TestThatGetKnownDeviceReturns200(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"default"})
	resp, body := testRequest(server, http.MethodGet, "/api/v0/devices/intern-a81758fffe06bfa3", token, nil)

	d := struct {
		Data struct {
			DevEui string `json:"sensorID"`
		} `json:"data"`
	}{}
	json.Unmarshal([]byte(body), &d)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal("a81758fffe06bfa3", d.Data.DevEui)
	//is.Equal(body, `{"devEUI":"a81758fffe06bfa3","deviceID":"intern-a81758fffe06bfa3","name":"name-a81758fffe06bfa3","description":"desc-a81758fffe06bfa3","location":{"latitude":62.3916,"longitude":17.30723,"altitude":0},"environment":"water","types":["urn:oma:lwm2m:ext:3303","urn:oma:lwm2m:ext:3302","urn:oma:lwm2m:ext:3301"],"sensorType":{"id":1,"name":"elsys","description":"","interval":3600},"lastObserved":"0001-01-01T00:00:00Z","active":true,"tenant":"default","status":{"batteryLevel":0,"statusCode":0,"timestamp":""},"interval":60}`)
}

func TestThatGetKnownDeviceMarshalToType(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"default"})
	resp, body := testRequest(server, http.MethodGet, "/api/v0/devices/intern-a81758fffe06bfa3", token, nil)

	d := struct {
		Data types.Device
	}{}
	json.Unmarshal([]byte(body), &d)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal("a81758fffe06bfa3", d.Data.SensorID)
	is.Equal("default", d.Data.Tenant)
}

func TestThatGetKnownDeviceByEUIFromNonAllowedTenantReturns404(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"wrongtenant"})
	resp, _ := testRequest(server, http.MethodGet, "/api/v0/devices?devEUI=a81758fffe06bfa3", token, nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func TestThatGetKnownDeviceFromNonAllowedTenantReturns404(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"wrongtenant"})
	resp, _ := testRequest(server, http.MethodGet, "/api/v0/devices/intern-a81758fffe06bfa3", token, nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func setupTest(t *testing.T) (*http.ServeMux, *is.I) {
	is := is.New(t)
	ctx := context.Background()

	config := storage.NewConfig(
		"localhost",
		"postgres",
		"password",
		"5432",
		"postgres",
		"disable",
	)

	p, err := storage.NewPool(ctx, config)
	if err != nil {
		t.Log("could not connect to postgres, will skip test")
		t.SkipNow()
	}

	s := storage.NewWithPool(p)

	msgCtx := messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
		PublishOnTopicFunc: func(ctx context.Context, message messaging.TopicMessage) error {
			return nil
		},
	}

	cfg := &devicemanagement.DeviceManagementConfig{}
	is.NoErr(yaml.Unmarshal([]byte(configYaml), cfg))

	app := devicemanagement.New(s, &msgCtx, io.NopCloser(strings.NewReader(configYaml)))
	err = app.Seed(context.Background(), io.NopCloser(strings.NewReader(csvMock)), []string{"default"})
	is.NoErr(err)

	policies := bytes.NewBufferString(opaModule)
	mux := http.NewServeMux()
	api.RegisterHandlers(ctx, mux, policies, app, nil)

	return mux, is
}

func testRequest(ts *httptest.Server, method, path string, token string, body io.Reader) (*http.Response, string) {
	req, _ := http.NewRequest(method, ts.URL+path, body)

	if len(token) > 0 {
		req.Header.Add("Authorization", "Bearer "+token)
	}

	resp, _ := http.DefaultClient.Do(req)
	respBody, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	return resp, string(respBody)
}

func createJWTWithTenants(tenants []string) string {
	tokenAuth := jwtauth.New("HS256", []byte("secret"), nil)
	_, tokenString, _ := tokenAuth.Encode(map[string]any{"user_id": 123, "azp": "diwise-frontend", "tenants": tenants})
	return tokenString
}

const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default;60;source
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;default;60;
a81758fffe04d83f;intern-a81758fffe04d83f;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe04d83f;desc-a81758fffe04d83f;true;default;60;`

const configYaml string = `
deviceprofiles:
  - name: qalcosonic
    decoder: qalcosonic
    interval: 3600
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3424
      - urn:oma:lwm2m:ext:3303
  - name: axsensor
    decoder: axsensor
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3330
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3327
      - urn:oma:lwm2m:ext:3303
types:
  - urn : urn:oma:lwm2m:ext:3
    name: Device 
  - urn: urn:oma:lwm2m:ext:3303
    name: Temperature
`

const opaModule string = `
#
# Use https://play.openpolicyagent.org for easier editing/validation of this policy file
#

package example.authz

default allow := false

allow = response {
    is_valid_token

    input.method == "GET"
    pathstart := array.slice(input.path, 0, 3)
    pathstart == ["api", "v0", "devices"]

    token.payload.azp == "diwise-frontend"

    response := {
        "tenants": token.payload.tenants
    }
}

is_valid_token {
    1 == 1
}

token := {"payload": payload} {
    [_, payload, _] := io.jwt.decode(input.token)
}
`
