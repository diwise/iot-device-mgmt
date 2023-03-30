package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/service"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

const noToken string = ""

func TestThatHealthEndpointReturns204NoContent(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, _ := testRequest(is, server, http.MethodGet, "/health", noToken, nil)

	is.Equal(resp.StatusCode, http.StatusNoContent)
}

func TestThatGetUnknownDeviceReturns404(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"default"})
	resp, _ := testRequest(is, server, http.MethodGet, "/api/v0/devices/nosuchdevice", token, nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func TestThatGetKnownDeviceByEUIReturns200(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"default"})
	resp, body := testRequest(is, server, http.MethodGet, "/api/v0/devices?devEUI=a81758fffe06bfa3", token, nil)

	d := []struct {
		DevEui string `json:"sensorID"`
	}{}
	json.Unmarshal([]byte(body), &d)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal("a81758fffe06bfa3", d[0].DevEui)
	//is.Equal(body, `[{"devEUI":"a81758fffe06bfa3","deviceID":"intern-a81758fffe06bfa3","name":"name-a81758fffe06bfa3","description":"desc-a81758fffe06bfa3","location":{"latitude":62.3916,"longitude":17.30723,"altitude":0},"environment":"water","types":["urn:oma:lwm2m:ext:3303","urn:oma:lwm2m:ext:3302","urn:oma:lwm2m:ext:3301"],"sensorType":{"id":1,"name":"elsys_codec","description":"","interval":3600},"lastObserved":"0001-01-01T00:00:00Z","active":true,"tenant":"default","status":{"batteryLevel":0,"statusCode":0,"timestamp":""},"interval":60}]`)
}

func TestThatGetKnownDeviceReturns200(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"default"})
	resp, body := testRequest(is, server, http.MethodGet, "/api/v0/devices/intern-a81758fffe06bfa3", token, nil)

	d := struct {
		DevEui string `json:"sensorID"`
	}{}
	json.Unmarshal([]byte(body), &d)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal("a81758fffe06bfa3", d.DevEui)
	//is.Equal(body, `{"devEUI":"a81758fffe06bfa3","deviceID":"intern-a81758fffe06bfa3","name":"name-a81758fffe06bfa3","description":"desc-a81758fffe06bfa3","location":{"latitude":62.3916,"longitude":17.30723,"altitude":0},"environment":"water","types":["urn:oma:lwm2m:ext:3303","urn:oma:lwm2m:ext:3302","urn:oma:lwm2m:ext:3301"],"sensorType":{"id":1,"name":"elsys_codec","description":"","interval":3600},"lastObserved":"0001-01-01T00:00:00Z","active":true,"tenant":"default","status":{"batteryLevel":0,"statusCode":0,"timestamp":""},"interval":60}`)
}

func TestThatGetKnownDeviceByEUIFromNonAllowedTenantReturns404(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"wrongtenant"})
	resp, _ := testRequest(is, server, http.MethodGet, "/api/v0/devices?devEUI=a81758fffe06bfa3", token, nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func TestThatGetKnownDeviceFromNonAllowedTenantReturns404(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	token := createJWTWithTenants([]string{"wrongtenant"})
	resp, _ := testRequest(is, server, http.MethodGet, "/api/v0/devices/intern-a81758fffe06bfa3", token, nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func setupTest(t *testing.T) (*chi.Mux, *is.I) {
	is := is.New(t)
	log := zerolog.Logger{}

	db, err := database.NewDeviceRepository(database.NewSQLiteConnector(log))
	is.NoErr(err)

	err = db.Seed(context.Background(), "devices.csv", bytes.NewBuffer([]byte(csvMock)))
	is.NoErr(err)

	app := service.New(db, &messaging.MsgContextMock{})
	router := router.New("testService")

	policies := bytes.NewBufferString(opaModule)
	api.RegisterHandlers(log, router, policies, app, &alarms.AlarmServiceMock{})

	return router, is
}

func testRequest(is *is.I, ts *httptest.Server, method, path string, token string, body io.Reader) (*http.Response, string) {
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

const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default;60
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;default;60
a81758fffe04d83f;intern-a81758fffe04d83f;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe04d83f;desc-a81758fffe04d83f;true;default;60`

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
