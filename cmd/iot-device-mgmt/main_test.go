package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/go-chi/chi/v5"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestThatHealthEndpointReturns204NoContent(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, _ := testRequest(is, server, http.MethodGet, "/health", nil)

	is.Equal(resp.StatusCode, http.StatusNoContent)
}

func TestThatGetUnknownDeviceReturns404(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, _ := testRequest(is, server, http.MethodGet, "/api/v0/devices/nosuchdevice", nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func TestThatGetKnownDeviceByEUIReturns200(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, body := testRequest(is, server, http.MethodGet, "/api/v0/devices?devEUI=a81758fffe06bfa3", nil)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal(body, `[{"id":"intern-a81758fffe06bfa3","name":"name-a81758fffe06bfa3","description":"desc-a81758fffe06bfa3","latitude":62.3916,"longitude":17.30723,"environment":"water","types":["urn:oma:lwm2m:ext:3303","urn:oma:lwm2m:ext:3302","urn:oma:lwm2m:ext:3301"],"sensorType":"Elsys_Codec","lastObserved":"0001-01-01T00:00:00Z","active":true}]`)
}

func TestThatGetKnownDeviceReturns200(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, body := testRequest(is, server, http.MethodGet, "/api/v0/devices/intern-a81758fffe06bfa3", nil)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal(body, `{"id":"intern-a81758fffe06bfa3","name":"name-a81758fffe06bfa3","description":"desc-a81758fffe06bfa3","latitude":62.3916,"longitude":17.30723,"environment":"water","types":["urn:oma:lwm2m:ext:3303","urn:oma:lwm2m:ext:3302","urn:oma:lwm2m:ext:3301"],"sensorType":"Elsys_Codec","lastObserved":"0001-01-01T00:00:00Z","active":true}`)
}

func setupTest(t *testing.T) (*chi.Mux, *is.I) {
	is := is.New(t)
	log := zerolog.Logger{}
	db, err := database.SetUpNewDatabase(log, bytes.NewBuffer([]byte(csvMock)))
	is.NoErr(err)
	app := application.New(db)
	router := router.New("testService")
	api.RegisterHandlers(log, router, app)

	return router, is
}

func testRequest(is *is.I, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, _ := http.NewRequest(method, ts.URL+path, body)
	resp, _ := http.DefaultClient.Do(req)
	respBody, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	return resp, string(respBody)
}

const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true
a81758fffe04d83f;intern-a81758fffe04d83f;0.0;0.0;ground;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe04d83f;desc-a81758fffe04d83f;true`
