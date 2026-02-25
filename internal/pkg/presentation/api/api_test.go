package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestExportDevices(t *testing.T) {
	is, _, _, s, _, ctx, mux := testSetup(t)
	server := httptest.NewServer(mux)
	defer server.Close()

	devices := []types.Device{}

	s.QueryFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data:       devices,
			Count:      uint64(len(devices)),
			Offset:     0,
			Limit:      uint64(len(devices)),
			TotalCount: uint64(len(devices)),
		}, nil
	}

	s.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, error) {
		return types.Device{}, storage.ErrNoRows
	}

	s.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		devices = append(devices, d)
		return nil
	}

	storage.SeedDevices(ctx, s, io.NopCloser(strings.NewReader(csvMock)), []string{"default"})

	req, _ := http.NewRequest(http.MethodGet, server.URL+"/api/v0/devices?export=true", nil)
	req.Header.Add("Authorization", "Bearer ????")
	req.Header.Add("accept", "text/csv")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)

	b, err := io.ReadAll(res.Body)
	is.NoErr(err)

	csv := strings.Split(string(b), "\n")

	is.Equal("devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source;metadata", csv[0])
	is.Equal("a81758fffe06bfa3;intern-a81758fffe06bfa3;62.391600;17.307230;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;elsys_codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default;0;source;key=value", csv[1])
}

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

func TestPatchDeviceWithPayloadHandler(t *testing.T) {
	putJson := `
{
    "active": true,
    "description": "alla utom sensor\\r\\njord\\r\\n3601",
    "deviceID": "defca053-6ec8-5430-a318-92bf7fcb854f",
    "deviceProfile": "elsys",
    "environment": "soil",
    "interval": "3601",
    "latitude": 59.373227,
    "longitude": 18.632813,
    "name": "EMS_Desk_05",
    "tenant": "default",
    "types": ["urn:oma:lwm2m:ext:3301", "urn:oma:lwm2m:ext:3200", "urn:oma:lwm2m:ext:3304", "urn:oma:lwm2m:ext:3428", "urn:oma:lwm2m:ext:3302", "urn:oma:lwm2m:ext:3303"]
}`

	is, _, _, repo, _, _, mux := testSetup(t)

	repo.QueryFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data: []types.Device{
				{
					Active:   false,
					SensorID: uuid.NewString(),
					DeviceID: "defca053-6ec8-5430-a318-92bf7fcb854f",
				},
			},
			Count:      1,
			Offset:     0,
			Limit:      1,
			TotalCount: 1,
		}, nil
	}

	repo.SetDeviceFunc = func(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error {
		return nil
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPatch, server.URL+"/api/v0/devices/defca053-6ec8-5430-a318-92bf7fcb854f", strings.NewReader(putJson))

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(http.StatusOK, res.StatusCode)
}

func TestCreateNewDeviceHandler(t *testing.T) {
	deviceJson := `
{
    "active": false,
    "sensorID": "24e124743c211337",
    "deviceID": "ecff5911-e771-5c5c-b6eb-e9f1bc932195",
    "tenant": "default",
    "name": "SN-WS302-01",
    "description": "",
    "location": {
        "latitude": 0,
        "longitude": 0
    },
    "types": null,
    "deviceProfile": {
        "name": "unknown",
        "decoder": "unknown",
        "interval": 0
    },
    "deviceStatus": {
        "batteryLevel": 0,
        "observedAt": "0001-01-01T00:00:00Z"
    },
    "deviceState": {
        "online": false,
        "state": 0,
        "observedAt": "0001-01-01T00:00:00Z"
    }
}`

	is, _, _, _, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	req, _ := http.NewRequest(http.MethodPost, server.URL+"/api/v0/devices", strings.NewReader(deviceJson))

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(http.StatusCreated, res.StatusCode)
}

func TestGetAlarmsHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	deviceID := uuid.NewString()

	repo.GetAlarmsFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Alarms], error) {
		return types.Collection[types.Alarms]{
			Data: []types.Alarms{
				{
					DeviceID:   deviceID,
					AlarmTypes: []string{alarms.AlarmDeviceNotObserved},
					ObservedAt: time.Now().UTC(),
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
		Data []types.Alarms
	}{}
	b, _ := io.ReadAll(res.Body)
	json.Unmarshal(b, &response)

	is.Equal(response.Data[0].DeviceID, deviceID)
}

func TestGetAlarmsWithFilterHandler(t *testing.T) {
	is, _, _, repo, _, _, mux := testSetup(t)

	server := httptest.NewServer(mux)
	defer server.Close()

	deviceID := uuid.NewString()

	endpoint := fmt.Sprintf("%s/api/v0/alarms?alarmtype=%s", server.URL, alarms.AlarmDeviceNotObserved)

	repo.GetAlarmsFunc = func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Alarms], error) {
		return types.Collection[types.Alarms]{
			Data: []types.Alarms{
				{
					DeviceID:   deviceID,
					AlarmTypes: []string{alarms.AlarmDeviceNotObserved},
				},
			},
			Count:      1,
			Offset:     0,
			Limit:      1,
			TotalCount: 1,
		}, nil
	}

	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer ????")

	res, err := http.DefaultClient.Do(req)
	is.NoErr(err)

	is.Equal(200, res.StatusCode)

	response := struct {
		Data []types.Alarms
	}{}
	b, _ := io.ReadAll(res.Body)
	json.Unmarshal(b, &response)

	containsAlartType := slices.ContainsFunc(response.Data[0].AlarmTypes, func(a string) bool { return a == alarms.AlarmDeviceNotObserved })

	is.True(containsAlartType)
	is.Equal(response.Data[0].DeviceID, deviceID)

	calls := repo.GetAlarmsCalls()
	c := (&storage.Condition{})
	for _, f := range calls[0].Conditions {
		c = f(c)
	}
	is.Equal(c.AlarmType, "device_not_observed")
}

func testSetup(t *testing.T) (*is.I, devicemanagement.DeviceManagement, *messaging.MsgContextMock, *storage.StoreMock, *slog.Logger, context.Context, *http.ServeMux) {
	is := is.New(t)
	ctx := context.Background()

	msgCtx := &messaging.MsgContextMock{
		RegisterTopicMessageHandlerFunc: func(routingKey string, handler messaging.TopicMessageHandler) error {
			return nil
		},
	}

	db := &storage.StoreMock{
		GetDeviceBySensorIDFunc: func(ctx context.Context, sensorID string) (types.Device, error) {
			return types.Device{
				SensorID: sensorID,
				DeviceID: "intern-" + sensorID,
			}, nil
		},
		IsSeedExistingDevicesEnabledFunc: func(ctx context.Context) bool {
			return true
		},
		CreateOrUpdateDeviceFunc: func(ctx context.Context, d types.Device) error {
			return nil
		},
		QueryFunc: func(ctx context.Context, conditions ...storage.ConditionFunc) (types.Collection[types.Device], error) {
			return types.Collection[types.Device]{}, nil
		},
		SetDeviceFunc: func(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error {
			return nil
		},
		SetDeviceProfileFunc: func(ctx context.Context, deviceID string, dp types.DeviceProfile) error {
			return nil
		},
		SetDeviceProfileTypesFunc: func(ctx context.Context, deviceID string, typesMoqParam []types.Lwm2mType) error {
			return nil
		},
		AddAlarmFunc: func(ctx context.Context, deviceID string, a types.AlarmDetails) error {
			return nil
		},
		GetDeviceStatusFunc: func(ctx context.Context, deviceID string, conditions ...storage.ConditionFunc) (types.Collection[types.DeviceStatus], error) {
			return types.Collection[types.DeviceStatus]{}, nil
		},
	}
	repo := devicemanagement.NewStorage(db)
	dm := devicemanagement.New(repo, msgCtx, &devicemanagement.DeviceManagementConfig{})
	arepo := alarms.NewStorage(db)
	as := alarms.New(arepo, msgCtx, &alarms.AlarmServiceConfig{})

	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	mux := http.NewServeMux()
	RegisterHandlers(ctx, mux, strings.NewReader(policiesMock), dm, as, db)

	return is, dm, msgCtx, db, log, ctx, mux
}

const csvMock string = `sensor_id;device_id;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source;metadata
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default;60;source;key=value
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;_default;60;source;
1234;intern-1234;0.0;0.0;air;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3304;enviot;name-1234;desc-1234;true;_test;60;källa;
5678;intern-5678;0.0;0.0;soil;urn:oma:lwm2m:ext:3303;enviot;name-5678;desc-5678;true;_test;60;;
`

const policiesMock string = `
package example.authz

# See https://www.openpolicyagent.org/docs/latest/policy-reference/ to learn more about rego

default allow := false

allow = response if {
	response := {
		"access": {
            "default": [
                "devices.create",
                "devices.read",
                "devices.update"
            ]
        }
	}
}`
