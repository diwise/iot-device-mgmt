package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestGetDevicesWithinBoundsIsCalledIfBoundsExistInQuery(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)
	server := httptest.NewServer(mux)
	defer server.Close()

	c := 0
	repo.QueryFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
		c = len(conditions)
		return types.Collection[types.Device]{}, nil
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v0/devices?bounds=%5B62.387942893965395%2C17.2897328765558%3B62.3955798771803%2C17.33788389279115%5D", nil)
	req.Header.Add("Authorization", "Bearer ????")
	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)

	is.Equal(2, c)
}

func TestCreateDeviceHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	filePath := "devices.csv"
	fieldName := "fileupload"
	body := new(bytes.Buffer)

	part := multipart.NewWriter(body)

	w, err := part.CreateFormFile(fieldName, filePath)
	is.NoErr(err)

	_, err = io.Copy(w, strings.NewReader(csvMock))
	is.NoErr(err)

	part.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v0/devices", body)
	req.Header.Add("Content-Type", part.FormDataContentType())
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(201, res.StatusCode)
	is.Equal(1, len(repo.CreateOrUpdateDeviceCalls()))
}

func TestGetDeviceHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	repo.QueryFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data: []types.Device{
				{
					DeviceID: "33788389279",
				},
			},
			Count: 1}, nil
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v0/devices/33788389279", nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)

	response := struct {
		Data types.Device
	}{}

	b, _ := io.ReadAll(res.Body)
	json.Unmarshal(b, &response)

	is.Equal("33788389279", response.Data.DeviceID)
}

func TestGetDeviceStatusHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	repo.QueryFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data: []types.Device{
				{
					DeviceID: "33788389279",
				},
			},
			Count: 1}, nil
	}

	repo.GetDeviceStatusFunc = func(ctx context.Context, deviceID string) (types.Collection[types.DeviceStatus], error) {
		return types.Collection[types.DeviceStatus]{}, nil
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v0/devices/33788389279/status", nil)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)
	is.Equal(1, len(repo.GetDeviceStatusCalls()))
}

func TestPatchDeviceHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	repo.QueryFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data: []types.Device{
				{
					DeviceID: "33788389279",
				},
			},
			Count: 1}, nil
	}
	repo.SetDeviceFunc = func(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error {
		return nil
	}

	body := strings.NewReader(`{"name":"newname"}`)

	req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/v0/devices/33788389279", body)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)
	is.Equal(1, len(repo.SetDeviceCalls()))
}

func TestPutDeviceHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	repo.QueryFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data: []types.Device{
				{
					DeviceID: "33788389279",
				},
			},
			Count: 1}, nil
	}
	repo.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		return nil
	}

	body := strings.NewReader(`
		{ 
			"deviceID" : "33788389279",
			"tenant" : "default" 
		}
	`)

	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v0/devices/33788389279", body)

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)
	is.Equal(1, len(repo.CreateOrUpdateDeviceCalls()))
}

func TestGetAlarmsHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	deviceID := uuid.NewString()

	repo.GetAlarmsFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Alarm], error) {
		return types.Collection[types.Alarm]{
			Data: []types.Alarm{
				{
					DeviceID:    deviceID,
					AlarmType:   alarms.AlarmDeviceNotObserved,
					Description: "",
					ObservedAt:  time.Now().UTC(),
					Severity:    types.AlarmSeverityLow,
				},
			},
			Count:      1,
			Offset:     0,
			Limit:      1,
			TotalCount: 1,
		}, nil
	}

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v0/alarms", nil)

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)

	response := struct {
		Data []types.Alarm
	}{}
	b, _ := io.ReadAll(res.Body)
	json.Unmarshal(b, &response)

	is.Equal(response.Data[0].DeviceID, deviceID)
}

func testSetup(t *testing.T) (*is.I, devicemanagement.DeviceManagement, *messaging.MsgContextMock, *storage.StoreMock, *slog.Logger, context.Context, *http.ServeMux) {
	is := is.New(t)
	ctx := context.Background()

	msgCtx := &messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
	}

	cfg, _ := devicemanagement.NewConfig(io.NopCloser(strings.NewReader(configYaml)))

	db := &storage.StoreMock{
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
		QueryFunc: func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
			return types.Collection[types.Device]{}, nil
		},
	}
	repo := devicemanagement.NewDeviceStorage(db)
	dm := devicemanagement.New(repo, msgCtx, cfg)
	arepo := alarms.NewAlarmStorage(db)
	as := alarms.New(arepo, msgCtx)

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	RegisterHandlers(ctx, mux, strings.NewReader(policiesMock), dm, as, db)

	return is, dm, msgCtx, db, log, ctx, mux
}

const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default;60;source
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;_default;60;source
1234;intern-1234;0.0;0.0;air;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3304;enviot;name-1234;desc-1234;true;_test;60;källa
5678;intern-5678;0.0;0.0;soil;urn:oma:lwm2m:ext:3303;enviot;name-5678;desc-5678;true;_test;60;
`

const policiesMock string = `
package example.authz

# See https://www.openpolicyagent.org/docs/latest/policy-reference/ to learn more about rego

default allow := false

allow = response {
	response := {
		"tenants": ["default"]
	}
}`

const configYaml string = `
deviceprofiles:
  - name: axsensor
    decoder: axsensor
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3327
      - urn:oma:lwm2m:ext:3330
  - name: elsys
    decoder: elsys
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3200
      - urn:oma:lwm2m:ext:3301
      - urn:oma:lwm2m:ext:3302
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3428
  - name: elsys_codec
    decoder: elsys_codec
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3200      
      - urn:oma:lwm2m:ext:3301
      - urn:oma:lwm2m:ext:3302
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3428
  - name: elt_2_hp
    decoder: elt_2_hp
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3200
      - urn:oma:lwm2m:ext:3301
      - urn:oma:lwm2m:ext:3302
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3428
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
      - urn:oma:lwm2m:ext:3200
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3304
      - urn:oma:lwm2m:ext:3330
      - urn:oma:lwm2m:ext:3428
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
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3424
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
      - urn:oma:lwm2m:ext:3302
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3304
  - name: sensefarm
    decoder: sensefarm
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3323
      - urn:oma:lwm2m:ext:3327
  - name: vegapuls_air_41
    decoder: vegapuls_air_41
    interval: 3600 
    types:
      - urn:oma:lwm2m:ext:3
      - urn:oma:lwm2m:ext:3303
      - urn:oma:lwm2m:ext:3330
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
  - urn : urn:oma:lwm2m:ext:3
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
`
