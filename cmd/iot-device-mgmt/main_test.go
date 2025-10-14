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

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"

	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/go-chi/jwtauth/v5"
	"github.com/matryer/is"
)

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

	exisitingDeviceUpdateFlag := true

	config := storage.NewConfig(
		"localhost",
		"postgres",
		"password",
		"5432",
		"postgres",
		"disable",
		exisitingDeviceUpdateFlag,
	)

	p, err := storage.NewPool(ctx, config)
	if err != nil {
		t.Log("could not connect to postgres, will skip test")
		t.SkipNow()
	}

	s := storage.NewWithPool(p)

	err = s.Initialize(ctx)
	if err != nil {
		t.Log("could not initialize storage, will skip test")
		t.SkipNow()
	}

	msgCtx := messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
		PublishOnTopicFunc: func(ctx context.Context, message messaging.TopicMessage) error {
			return nil
		},
	}

	cfg, _ := parseExternalConfigFile(context.Background(), io.NopCloser(strings.NewReader(configYaml)))
	dm := devicemanagement.New(s, &msgCtx, &cfg.DeviceManagementConfig)
	as := alarms.New(alarms.NewStorage(s), &msgCtx, &cfg.AlarmServiceConfig)

	err = storage.SeedLwm2mTypes(ctx, s, dm.Config().Types)
	is.NoErr(err)

	err = storage.SeedDeviceProfiles(ctx, s, dm.Config().DeviceProfiles)
	is.NoErr(err)

	err = storage.SeedDevices(ctx, s, io.NopCloser(strings.NewReader(csvMock)), []string{"default"})
	is.NoErr(err)

	policies := bytes.NewBufferString(opaModule)
	mux := http.NewServeMux()
	api.RegisterHandlers(ctx, mux, policies, dm, as, s)

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
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;elsys;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default;60;source
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;elsys;name-a81758fffe051d00;desc-a81758fffe051d00;true;default;60;
a81758fffe04d83f;intern-a81758fffe04d83f;0.0;0.0;air;urn:oma:lwm2m:ext:3303;elsys;name-a81758fffe04d83f;desc-a81758fffe04d83f;true;default;60;`

const configYaml string = `
devicemanagement:
  deviceprofiles:
    - name: axsensor
      decoder: axsensor
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3330
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3327
        - urn:oma:lwm2m:ext:3303
    - name: elsys
      decoder: elsys
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3303
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3301
        - urn:oma:lwm2m:ext:3428
        - urn:oma:lwm2m:ext:3302
        - urn:oma:lwm2m:ext:3200
    - name: elt_2_hp
      decoder: elt_2_hp
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3303
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3301
        - urn:oma:lwm2m:ext:3428
        - urn:oma:lwm2m:ext:3302
        - urn:oma:lwm2m:ext:3200
    - name: enviot
      decoder: enviot
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3303
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3330
    - name: milesight
      decoder: milesight
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3428
        - urn:oma:lwm2m:ext:3330
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3303
    - name: niab-fls
      decoder: niab-fls
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3303
        - urn:oma:lwm2m:ext:3330
    - name: qalcosonic
      decoder: qalcosonic
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3424
        - urn:oma:lwm2m:ext:3303
    - name: senlabt
      decoder: senlabt
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3303
    - name: sensative
      decoder: sensative
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3303
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3302
    - name: sensefarm
      decoder: sensefarm
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3327
        - urn:oma:lwm2m:ext:3323
    - name: vegapuls_air_41
      decoder: vegapuls_air_41
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3330
        - urn:oma:lwm2m:ext:3303
    - name: airquality
      decoder: airquality
      interval: 86400
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3303
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3428
    - name: virtual
      decoder: virtual
      interval: 3600
      types:
        - urn:oma:lwm2m:ext:3
        - urn:oma:lwm2m:ext:3200
        - urn:oma:lwm2m:ext:3301
        - urn:oma:lwm2m:ext:3302
        - urn:oma:lwm2m:ext:3303
        - urn:oma:lwm2m:ext:3304
        - urn:oma:lwm2m:ext:3323
        - urn:oma:lwm2m:ext:3327
        - urn:oma:lwm2m:ext:3328
        - urn:oma:lwm2m:ext:3330
        - urn:oma:lwm2m:ext:3331
        - urn:oma:lwm2m:ext:3350
        - urn:oma:lwm2m:ext:3411
        - urn:oma:lwm2m:ext:3424
        - urn:oma:lwm2m:ext:3428
        - urn:oma:lwm2m:ext:3434
        - urn:oma:lwm2m:ext:3435

  types:
    - urn: urn:oma:lwm2m:ext:3
      name: Device
    - urn: urn:oma:lwm2m:ext:3303
      name: Temperature
    - urn: urn:oma:lwm2m:ext:3304
      name: Humidity
    - urn: urn:oma:lwm2m:ext:3301
      name: Illuminance
    - urn: urn:oma:lwm2m:ext:3428
      name: AirQuality
    - urn: urn:oma:lwm2m:ext:3302
      name: Presence
    - urn: urn:oma:lwm2m:ext:3200
      name: DigitalInput
    - urn: urn:oma:lwm2m:ext:3330
      name: Distance
    - urn: urn:oma:lwm2m:ext:3327
      name: Conductivity
    - urn: urn:oma:lwm2m:ext:3323
      name: Pressure
    - urn: urn:oma:lwm2m:ext:3435
      name: FillingLevel
    - urn: urn:oma:lwm2m:ext:3424
      name: WaterMeter
    - urn: urn:oma:lwm2m:ext:3411
      name: Battery
    - urn: urn:oma:lwm2m:ext:3434
      name: PeopleCounter
    - urn: urn:oma:lwm2m:ext:3328
      name: Power
    - urn: urn:oma:lwm2m:ext:3331
      name: Energy
    - urn: urn:oma:lwm2m:ext:3350
      name: Stopwatch

alarmservice:
  alarmtypes:
    - name: backflow
      enabled: true
      type: application
      severity: 0
    - name: burst
      enabled: true
      type: application
      severity: 0
    - name: empty_spool
      enabled: true
      type: application
      severity: 0
    - name: freeze
      enabled: true
      type: application
      severity: 0
    - name: leak
      enabled: true
      type: application
      severity: 0
    - name: permanent_error
      enabled: true
      type: application
      severity: 0
    - name: power_low
      enabled: true
      type: application
      severity: 0
    - name: temporary_error
      enabled: true
      type: application
      severity: 0
    - name: downlink_gateway
      enabled: true
      type: system
      severity: 0
    - name: device_not_observed
      enabled: true
      type: system
      severity: 0
    - name: otaa
      enabled: true
      type: system
      severity: 0
    - name: uplink_fcnt_retransmission
      enabled: false
      type: system
      severity: 0
    - name: uplink_mic
      enabled: true
      type: system
      severity: 0
    - name: unknown
      enabled: true
      type: system
      severity: 0

watchdog:
  interval: 10
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
