package main

import (
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestSetup(t *testing.T) {
	is := is.New(t)
	r := createAppAndSetupRouter(zerolog.Logger{}, "test")
	server := httptest.NewServer(r)
	defer server.Close()

	resp, _ := testRequest(is, server, http.MethodGet, "/health", nil)

	is.Equal(resp.StatusCode, http.StatusNoContent)
}

func TestThatGetUnknownDeviceReturns404(t *testing.T) {
	is := is.New(t)
	r := createAppAndSetupRouter(zerolog.Logger{}, "test")
	server := httptest.NewServer(r)
	defer server.Close()

	resp, _ := testRequest(is, server, http.MethodGet, "/api/v0/devices/nosuchdevice", nil)

	is.Equal(resp.StatusCode, http.StatusNotFound)
}

func TestThatGetKnownDeviceReturns200(t *testing.T) {
	is := is.New(t)
	r := createAppAndSetupRouter(zerolog.Logger{}, "test")
	server := httptest.NewServer(r)
	defer server.Close()

	resp, body := testRequest(is, server, http.MethodGet, "/api/v0/devices/a81758fffe06bfa3", nil)

	is.Equal(resp.StatusCode, http.StatusOK)
	is.Equal(body, "{\"id\":\"intern-a81758fffe06bfa3\",\"types\":[\"urn:oma:lwm2m:ext:3303\"]}")
}

func testRequest(is *is.I, ts *httptest.Server, method, path string, body io.Reader) (*http.Response, string) {
	req, _ := http.NewRequest(method, ts.URL+path, body)
	resp, _ := http.DefaultClient.Do(req)
	respBody, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()

	return resp, string(respBody)
}
