package api

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/infrastructure/storage"
	"github.com/diwise/messaging-golang/pkg/messaging"

	conditions "github.com/diwise/iot-device-mgmt/internal/pkg/types"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func TestApi(t *testing.T) {
	ctx := t.Context()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	msgMock := &messaging.MsgContextMock{}
	ds := &devicemanagement.DeviceStorageMock{}

	dm := devicemanagement.New(ds, msgMock, &devicemanagement.Config{})
	as := alarms.AlarmServiceMock{}

	mux := http.NewServeMux()
	RegisterHandlers(ctx, mux, policies, dm, &as)

	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("GET /devices", func(t *testing.T) {
		testQueryDevices(t, server.URL, ds)
	})

	t.Run("GET /devices?devEUI=test-sensor-1", func(t *testing.T) {
		testQueryDevicesBySensorID(t, server.URL, ds)
	})

	t.Run("GET /devices/test-device-1", func(t *testing.T) {
		testGetDevice(t, server.URL, ds)
	})

	t.Run("GET /devices/test-device-1/status", func(t *testing.T) {
		testDeviceStatus(t, server.URL, ds)
	})

	t.Run("GET /devices/test-device-1/alarms", func(t *testing.T) {
		testDeviceAlarms(t, server.URL, ds)
	})

	t.Run("GET /devices/test-device-1/measurements", func(t *testing.T) {
		testDeviceMeasurements(t, server.URL, ds)
	})

	t.Run("POST /devices", func(t *testing.T) {
		testCreateDevice(t, server.URL, ds)
	})

	t.Run("POST /devices+multiPart", func(t *testing.T) {
		testCreateDevices(t, server.URL, ds)
	})

}

func testQueryDevices(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.QueryFunc = func(ctx context.Context, conds ...conditions.ConditionFunc) (types.Collection[types.Device], error) {
		collection := types.Collection[types.Device]{
			Data: []types.Device{testDevice},
		}
		return collection, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/devices", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"sensorID":"test-sensor-1"`) {
		t.Fatalf("expected response to contain sensorID 'test-sensor-1', got %s", string(body))
	}
}

func testQueryDevicesBySensorID(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, error) {
		return testDevice, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/devices?devEUI=test-sensor-1", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"sensorID":"test-sensor-1"`) {
		t.Fatalf("expected response to contain sensorID 'test-sensor-1', got %s", string(body))
	}
}

func testGetDevice(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.QueryFunc = func(ctx context.Context, conds ...conditions.ConditionFunc) (types.Collection[types.Device], error) {
		collection := types.Collection[types.Device]{
			Count: 1,
			Data:  []types.Device{testDevice},
		}
		return collection, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/devices/test-device-1", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"sensorID":"test-sensor-1"`) {
		t.Fatalf("expected response to contain sensorID 'test-sensor-1', got %s", string(body))
	}
}

func testDeviceStatus(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.GetDeviceStatusFunc = func(ctx context.Context, deviceID string, conds ...conditions.ConditionFunc) (types.Collection[types.SensorStatus], error) {
		collection := types.Collection[types.SensorStatus]{
			Data: []types.SensorStatus{
				{
					BatteryLevel: 45,
				},
			},
		}
		return collection, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/devices/test-device-1/status", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"batteryLevel":45`) {
		t.Fatalf("expected response to contain battery level '45', got %s", string(body))
	}
}

func testDeviceAlarms(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.GetDeviceAlarmsFunc = func(ctx context.Context, deviceID string) (types.Collection[types.AlarmDetails], error) {
		collection := types.Collection[types.AlarmDetails]{
			Data: []types.AlarmDetails{
				{
					AlarmType:   "battery_low",
					Description: "Battery is low",
					Severity:    2,
				},
			},
		}
		return collection, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/devices/test-device-1/alarms", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"alarmType":"battery_low"`) {
		t.Fatalf("expected response to contain alarm type 'battery_low', got %s", string(body))
	}
}

func testDeviceMeasurements(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.GetDeviceMeasurementsFunc = func(ctx context.Context, deviceID string, conds ...conditions.ConditionFunc) (types.Collection[types.Measurement], error) {
		collection := types.Collection[types.Measurement]{
			Data: []types.Measurement{
				{
					ID:    "-temp-1",
					Value: 21.5,
				},
			},
		}
		return collection, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/devices/test-device-1/measurements", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"value":21.5`) {
		t.Fatalf("expected response to contain measurement value '21.5', got %s", string(body))
	}
}

func testCreateDevice(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		return nil
	}

	ds.QueryFunc = func(ctx context.Context, conditionsMoqParam ...conditions.ConditionFunc) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data: []types.Device{},
		}, nil
	}

	payload := `{
		"deviceID": "new-device-1",
		"sensorID": "new-sensor-1",
		"tenant": "default"
	}`

	statusCode, _ := do(t, http.MethodPost, baseUrl+"/api/v0/devices", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", statusCode)
	}
}

func testCreateDevices(t *testing.T, baseUrl string, ds *devicemanagement.DeviceStorageMock) {
	ds.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		return nil
	}

	ds.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, error) {
		return types.Device{}, storage.ErrNoRows
	}

	ds.IsSeedExistingDevicesEnabledFunc = func(ctx context.Context) bool {
		return true
	}

	body, contentType := createMultipartFileUpload(t, "fileupload", "devices.csv", csvMock)

	statusCode, _ := do(t, http.MethodPost, baseUrl+"/api/v0/devices", body, map[string]string{"Content-Type": contentType})
	if statusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", statusCode)
	}
}

func createMultipartFileUpload(t *testing.T, fieldName, fileName, content string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("failed to create multipart file field: %v", err)
	}

	_, err = io.Copy(part, strings.NewReader(content))
	if err != nil {
		t.Fatalf("failed to write multipart content: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("failed to finalize multipart body: %v", err)
	}

	return body, writer.FormDataContentType()
}

func do(t *testing.T, method, url string, body io.Reader, headers ...map[string]string) (int, []byte) {
	req, _ := http.NewRequest(method, url, body)
	req.Header.Set("Authorization", "Bearer mock-token")

	for _, h := range headers {
		for k, v := range h {
			req.Header.Set(k, v)
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make request: %v", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	return resp.StatusCode, b
}

var testDevice = types.Device{SensorID: "test-sensor-1", DeviceID: "test-device-1", Tenant: "default"}

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

allow = response {
	response := {
		"tenants": ["default"]
	}
}`
