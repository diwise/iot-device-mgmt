package api

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
	"github.com/go-chi/chi/v5"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestSetup(t *testing.T) {
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

func TestThatGetKnownDeviceReturns200(t *testing.T) {
	r, is := setupTest(t)
	server := httptest.NewServer(r)
	defer server.Close()

	resp, body := testRequest(is, server, http.MethodGet, "/api/v0/devices/a81758fffe06bfa3", nil)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal(body, "{\"id\":\"intern-a81758fffe06bfa3\",\"types\":[\"urn:oma:lwm2m:ext:3303\"]}")
}

func setupTest(t *testing.T) (*chi.Mux, *is.I) {
	is := is.New(t)
	log := zerolog.Logger{}
	db, err := database.SetUpNewDatabase(log, bytes.NewBuffer([]byte(csvMock)))
	is.NoErr(err)
	app := application.New(db)
	router := router.New("testService")
	RegisterHandlers(log, router, app)

	return router, is
}

func testRequest(is *is.I, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, _ := http.NewRequest(method, ts.URL+path, body)
	resp, _ := http.DefaultClient.Do(req)
	respBody, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	return resp, string(respBody)
}

const csvMock string = `a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;where;3303
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;3303
a81758fffe04d83f;intern-a81758fffe04d83f;0.0;0.0;water;3303
a81758fffe0524f3;intern-a81758fffe0524f3;62.405478430159356;17.317086398702134;air;3303
a81758fffe04d84d;intern-a81758fffe04d84d;0.0;0.0;water;3303
a81758fffe04d843;intern-a81758fffe04d843;62.42270376259509;17.428565025329593;where;3303
a81758fffe04d851;intern-a81758fffe04d851;62.36956091265246;17.319844410529534;where;3303
a81758fffe051d02;intern-a81758fffe051d02;62.405478430159356;17.317086398702134;air;3303
a81758fffe04d856;intern-a81758fffe04d856;62.36956091265246;17.319844410529534;where;3303`
