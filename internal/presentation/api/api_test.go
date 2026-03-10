package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	"github.com/diwise/iot-device-mgmt/internal/application/devicemanagement"
	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devicemanagement/query"
	"github.com/diwise/iot-device-mgmt/internal/application/sensormanagement"
	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensormanagement/query"
	"github.com/diwise/messaging-golang/pkg/messaging"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func TestApi(t *testing.T) {
	ctx := t.Context()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	msgMock := &messaging.MsgContextMock{}
	mocks := newDeviceManagementMocks()
	sensorMocks := newSensorManagementMocks()

	dm := devicemanagement.New(mocks.reader, mocks.writer, mocks.statusWriter, mocks.profiles, msgMock, &devicemanagement.Config{})
	sm := sensormanagement.New(sensorMocks.reader, sensorMocks.writer)
	as := alarms.AlarmServiceMock{}

	mux := http.NewServeMux()
	RegisterHandlers(ctx, mux, policies, dm, sm, &as)

	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("GET /devices", func(t *testing.T) {
		testQueryDevices(t, server.URL, mocks)
	})

	t.Run("GET /sensors", func(t *testing.T) {
		testQuerySensors(t, server.URL, sensorMocks)
	})

	t.Run("GET /sensors?limit=invalid", func(t *testing.T) {
		testQuerySensorsWithInvalidLimit(t, server.URL)
	})

	t.Run("GET /sensors internal failure", func(t *testing.T) {
		testQuerySensorsInternalError(t, server.URL, sensorMocks)
	})

	t.Run("GET /sensors/test-sensor-standalone", func(t *testing.T) {
		testGetSensor(t, server.URL, sensorMocks)
	})

	t.Run("GET /sensors/missing", func(t *testing.T) {
		testGetSensorNotFound(t, server.URL, sensorMocks)
	})

	t.Run("GET /sensors/test-sensor-standalone internal failure", func(t *testing.T) {
		testGetSensorInternalError(t, server.URL, sensorMocks)
	})

	t.Run("GET /devices?limit=invalid", func(t *testing.T) {
		testQueryDevicesWithInvalidLimit(t, server.URL)
	})

	t.Run("GET /devices?devEUI=test-sensor-1", func(t *testing.T) {
		testQueryDevicesBySensorID(t, server.URL, mocks)
	})

	t.Run("GET /devices/test-device-1", func(t *testing.T) {
		testGetDevice(t, server.URL, mocks)
	})

	t.Run("GET /devices/test-device-1/status", func(t *testing.T) {
		testDeviceStatus(t, server.URL, mocks)
	})

	t.Run("GET /devices/test-device-1/alarms", func(t *testing.T) {
		testDeviceAlarms(t, server.URL, mocks)
	})

	t.Run("GET /devices/test-device-1/measurements", func(t *testing.T) {
		testDeviceMeasurements(t, server.URL, mocks)
	})

	t.Run("GET /alarms", func(t *testing.T) {
		testGetAlarms(t, server.URL, &as)
	})

	t.Run("GET /alarms?limit=invalid", func(t *testing.T) {
		testGetAlarmsWithInvalidLimit(t, server.URL)
	})

	t.Run("GET /admin/deviceprofiles", func(t *testing.T) {
		testGetDeviceProfiles(t, dm)
	})

	t.Run("GET /admin/deviceprofiles/missing", func(t *testing.T) {
		testGetDeviceProfilesNotFound(t, dm)
	})

	t.Run("GET /admin/deviceprofiles internal failure", func(t *testing.T) {
		testGetDeviceProfilesInternalError(t, dm)
	})

	t.Run("GET /admin/lwm2mtypes", func(t *testing.T) {
		testGetLwm2mTypes(t, dm)
	})

	t.Run("GET /admin/lwm2mtypes/missing", func(t *testing.T) {
		testGetLwm2mTypesNotFound(t, dm)
	})

	t.Run("GET /admin/lwm2mtypes internal failure", func(t *testing.T) {
		testGetLwm2mTypesInternalError(t, dm)
	})

	t.Run("POST /devices", func(t *testing.T) {
		testCreateDevice(t, server.URL, mocks)
	})

	t.Run("POST /sensors", func(t *testing.T) {
		testCreateSensor(t, server.URL, sensorMocks)
	})

	t.Run("POST /sensors duplicate", func(t *testing.T) {
		testCreateSensorDuplicate(t, server.URL, sensorMocks)
	})

	t.Run("POST /devices+multiPart", func(t *testing.T) {
		testCreateDevices(t, server.URL, mocks)
	})

	t.Run("PUT /devices/test-device-1", func(t *testing.T) {
		testUpdateDevice(t, server.URL, mocks)
	})

	t.Run("PUT /sensors/test-sensor-standalone", func(t *testing.T) {
		testUpdateSensor(t, server.URL, sensorMocks)
	})

	t.Run("PUT /sensors/missing", func(t *testing.T) {
		testUpdateSensorNotFound(t, server.URL, sensorMocks)
	})

	t.Run("PUT /sensors/test-sensor-standalone internal failure", func(t *testing.T) {
		testUpdateSensorInternalError(t, server.URL, sensorMocks)
	})

	t.Run("PUT /devices/missing-device", func(t *testing.T) {
		testUpdateDeviceNotFound(t, server.URL, mocks)
	})

	t.Run("PUT /devices/test-device-1 internal failure", func(t *testing.T) {
		testUpdateDeviceInternalError(t, server.URL, mocks)
	})

	t.Run("PATCH /devices/test-device-1", func(t *testing.T) {
		testPatchDevice(t, server.URL, mocks)
	})

	t.Run("PATCH /devices/test-device-1 invalid field type", func(t *testing.T) {
		testPatchDeviceInvalidField(t, server.URL, mocks)
	})

	t.Run("PATCH /devices/missing-device", func(t *testing.T) {
		testPatchDeviceNotFound(t, server.URL, mocks)
	})

	t.Run("PATCH /devices/test-device-1 internal failure", func(t *testing.T) {
		testPatchDeviceInternalError(t, server.URL, mocks)
	})

	t.Run("GET /alarms internal failure", func(t *testing.T) {
		testGetAlarmsInternalError(t, server.URL, &as)
	})

}

type adminAPIService struct {
	devicemanagement.DeviceAPIService
	profilesFunc   func(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error)
	lwm2mTypesFunc func(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error)
}

func (s adminAPIService) Profiles(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
	if s.profilesFunc != nil {
		return s.profilesFunc(ctx, name...)
	}

	return s.DeviceAPIService.Profiles(ctx, name...)
}

func (s adminAPIService) Lwm2mTypes(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
	if s.lwm2mTypesFunc != nil {
		return s.lwm2mTypesFunc(ctx, urn...)
	}

	return s.DeviceAPIService.Lwm2mTypes(ctx, urn...)
}

type deviceManagementMocks struct {
	reader       *devicemanagement.DeviceReaderMock
	writer       *devicemanagement.DeviceWriterMock
	statusWriter *devicemanagement.DeviceStatusWriterMock
	profiles     *devicemanagement.DeviceProfileStoreMock
}

type sensorReaderMock struct {
	QueryFunc func(ctx context.Context, query sensorquery.Sensors) (types.Collection[sensormanagement.Sensor], error)
	GetFunc   func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error)
}

func (m *sensorReaderMock) QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[sensormanagement.Sensor], error) {
	if m.QueryFunc == nil {
		panic("sensorReaderMock.QueryFunc is nil")
	}
	return m.QueryFunc(ctx, query)
}

func (m *sensorReaderMock) GetSensor(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
	if m.GetFunc == nil {
		panic("sensorReaderMock.GetFunc is nil")
	}
	return m.GetFunc(ctx, sensorID)
}

type sensorWriterMock struct {
	CreateFunc func(ctx context.Context, sensor sensormanagement.Sensor) error
	UpdateFunc func(ctx context.Context, sensor sensormanagement.Sensor) error
}

func (m *sensorWriterMock) CreateSensor(ctx context.Context, sensor sensormanagement.Sensor) error {
	if m.CreateFunc == nil {
		panic("sensorWriterMock.CreateFunc is nil")
	}
	return m.CreateFunc(ctx, sensor)
}

func (m *sensorWriterMock) UpdateSensor(ctx context.Context, sensor sensormanagement.Sensor) error {
	if m.UpdateFunc == nil {
		panic("sensorWriterMock.UpdateFunc is nil")
	}
	return m.UpdateFunc(ctx, sensor)
}

type sensorManagementMocks struct {
	reader *sensorReaderMock
	writer *sensorWriterMock
}

func newSensorManagementMocks() sensorManagementMocks {
	return sensorManagementMocks{
		reader: &sensorReaderMock{},
		writer: &sensorWriterMock{},
	}
}

func newNoopSensorAPIService() sensormanagement.SensorAPIService {
	mocks := sensorManagementMocks{
		reader: &sensorReaderMock{
			QueryFunc: func(ctx context.Context, query sensorquery.Sensors) (types.Collection[sensormanagement.Sensor], error) {
				return types.Collection[sensormanagement.Sensor]{}, nil
			},
			GetFunc: func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
				return sensormanagement.Sensor{}, false, nil
			},
		},
		writer: &sensorWriterMock{
			CreateFunc: func(ctx context.Context, sensor sensormanagement.Sensor) error { return nil },
			UpdateFunc: func(ctx context.Context, sensor sensormanagement.Sensor) error { return nil },
		},
	}

	return sensormanagement.New(mocks.reader, mocks.writer)
}

func newDeviceManagementMocks() deviceManagementMocks {
	return deviceManagementMocks{
		reader:       &devicemanagement.DeviceReaderMock{},
		writer:       &devicemanagement.DeviceWriterMock{},
		statusWriter: &devicemanagement.DeviceStatusWriterMock{},
		profiles:     &devicemanagement.DeviceProfileStoreMock{},
	}
}

func testQueryDevices(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
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

func testQuerySensors(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query sensorquery.Sensors) (types.Collection[sensormanagement.Sensor], error) {
		return types.Collection[sensormanagement.Sensor]{
			Data:       []sensormanagement.Sensor{testSensor},
			Count:      1,
			Offset:     0,
			Limit:      10,
			TotalCount: 1,
		}, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/sensors", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"sensorID":"test-sensor-standalone"`) {
		t.Fatalf("expected response to contain sensorID, got %s", string(body))
	}
	if !strings.Contains(string(body), `"decoder":"elsys"`) {
		t.Fatalf("expected response to contain sensor profile decoder, got %s", string(body))
	}
}

func testQuerySensorsWithInvalidLimit(t *testing.T, baseUrl string) {
	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors?limit=invalid", nil)
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", statusCode)
	}
}

func testQuerySensorsInternalError(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query sensorquery.Sensors) (types.Collection[sensormanagement.Sensor], error) {
		return types.Collection[sensormanagement.Sensor]{}, errors.New("query failed")
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testGetSensor(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return testSensor, true, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/sensors/test-sensor-standalone", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"sensorID":"test-sensor-standalone"`) {
		t.Fatalf("expected response to contain sensorID, got %s", string(body))
	}
}

func testGetSensorNotFound(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return sensormanagement.Sensor{}, false, nil
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors/missing", nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testGetSensorInternalError(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return sensormanagement.Sensor{}, false, errors.New("lookup failed")
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors/test-sensor-standalone", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testQueryDevicesWithInvalidLimit(t *testing.T, baseUrl string) {
	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/devices?limit=invalid", nil)
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", statusCode)
	}
}

func testQueryDevicesBySensorID(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, bool, error) {
		return testDevice, true, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/devices?devEUI=test-sensor-1", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"sensorID":"test-sensor-1"`) {
		t.Fatalf("expected response to contain sensorID 'test-sensor-1', got %s", string(body))
	}
}

func testGetDevice(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
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

func testDeviceStatus(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.GetDeviceStatusFunc = func(ctx context.Context, deviceID string, query dmquery.Status) (types.Collection[types.SensorStatus], error) {
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

func testDeviceAlarms(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Count: 1,
			Data:  []types.Device{testDevice},
		}, nil
	}
	mocks.reader.GetDeviceAlarmsFunc = func(ctx context.Context, deviceID string) (types.Collection[types.AlarmDetails], error) {
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

func testDeviceMeasurements(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.GetDeviceMeasurementsFunc = func(ctx context.Context, deviceID string, query dmquery.Measurements) (types.Collection[types.Measurement], error) {
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

func testGetAlarms(t *testing.T, baseUrl string, service *alarms.AlarmServiceMock) {
	service.AlarmsFunc = func(ctx context.Context, query alarmquery.Alarms) (types.Collection[types.Alarms], error) {
		return types.Collection[types.Alarms]{
			Data: []types.Alarms{{
				DeviceID:   "test-device-1",
				AlarmTypes: []string{"battery_low"},
			}},
			Count:      1,
			TotalCount: 1,
			Limit:      5,
			Offset:     0,
		}, nil
	}

	statusCode, body := do(t, http.MethodGet, baseUrl+"/api/v0/alarms", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"deviceID":"test-device-1"`) {
		t.Fatalf("expected response to contain device id 'test-device-1', got %s", string(body))
	}
	if !strings.Contains(string(body), `"battery_low"`) {
		t.Fatalf("expected response to contain alarm type 'battery_low', got %s", string(body))
	}
	if len(service.AlarmsCalls()) != 1 || !service.AlarmsCalls()[0].Query.ActiveOnly {
		t.Fatalf("expected alarms query to set activeOnly, got %+v", service.AlarmsCalls())
	}
}

func testGetAlarmsWithInvalidLimit(t *testing.T, baseUrl string) {
	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/alarms?limit=invalid", nil)
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", statusCode)
	}
}

func testGetAlarmsInternalError(t *testing.T, baseUrl string, service *alarms.AlarmServiceMock) {
	service.AlarmsFunc = func(ctx context.Context, query alarmquery.Alarms) (types.Collection[types.Alarms], error) {
		return types.Collection[types.Alarms]{}, errors.New("storage failure")
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/alarms", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testGetDeviceProfiles(t *testing.T, dm devicemanagement.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		profilesFunc: func(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
			return types.Collection[types.SensorProfile]{
				Data:       []types.SensorProfile{{Name: "profile-a", Decoder: "profile-a"}},
				Count:      1,
				Limit:      1,
				TotalCount: 1,
			}, nil
		},
	}

	mux := http.NewServeMux()
	if err := RegisterHandlers(t.Context(), mux, policies, service, newNoopSensorAPIService(), &alarms.AlarmServiceMock{}); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, body := do(t, http.MethodGet, server.URL+"/api/v0/admin/deviceprofiles", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"name":"profile-a"`) {
		t.Fatalf("expected response to contain profile name, got %s", string(body))
	}
}

func testGetDeviceProfilesNotFound(t *testing.T, dm devicemanagement.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		profilesFunc: func(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
			return types.Collection[types.SensorProfile]{}, devicemanagement.ErrDeviceProfileNotFound
		},
	}

	mux := http.NewServeMux()
	if err := RegisterHandlers(t.Context(), mux, policies, service, newNoopSensorAPIService(), &alarms.AlarmServiceMock{}); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/deviceprofiles/missing", nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testGetDeviceProfilesInternalError(t *testing.T, dm devicemanagement.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		profilesFunc: func(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
			return types.Collection[types.SensorProfile]{}, errors.New("profile store failed")
		},
	}

	mux := http.NewServeMux()
	if err := RegisterHandlers(t.Context(), mux, policies, service, newNoopSensorAPIService(), &alarms.AlarmServiceMock{}); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/deviceprofiles", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testGetLwm2mTypes(t *testing.T, dm devicemanagement.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		lwm2mTypesFunc: func(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
			return types.Collection[types.Lwm2mType]{
				Data:       []types.Lwm2mType{{Urn: "urn:test:1", Name: "Temperature"}},
				Count:      1,
				Limit:      1,
				TotalCount: 1,
			}, nil
		},
	}

	mux := http.NewServeMux()
	if err := RegisterHandlers(t.Context(), mux, policies, service, newNoopSensorAPIService(), &alarms.AlarmServiceMock{}); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, body := do(t, http.MethodGet, server.URL+"/api/v0/admin/lwm2mtypes", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}

	if !strings.Contains(string(body), `"urn":"urn:test:1"`) {
		t.Fatalf("expected response to contain lwm2m urn, got %s", string(body))
	}
}

func testGetLwm2mTypesNotFound(t *testing.T, dm devicemanagement.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		lwm2mTypesFunc: func(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
			return types.Collection[types.Lwm2mType]{}, devicemanagement.ErrDeviceProfileNotFound
		},
	}

	mux := http.NewServeMux()
	if err := RegisterHandlers(t.Context(), mux, policies, service, newNoopSensorAPIService(), &alarms.AlarmServiceMock{}); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/lwm2mtypes/missing", nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testGetLwm2mTypesInternalError(t *testing.T, dm devicemanagement.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		lwm2mTypesFunc: func(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
			return types.Collection[types.Lwm2mType]{}, errors.New("type store failed")
		},
	}

	mux := http.NewServeMux()
	if err := RegisterHandlers(t.Context(), mux, policies, service, newNoopSensorAPIService(), &alarms.AlarmServiceMock{}); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/lwm2mtypes", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testCreateDevice(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.writer.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		return nil
	}

	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
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

func testCreateSensor(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return sensormanagement.Sensor{}, false, nil
	}
	mocks.writer.CreateFunc = func(ctx context.Context, sensor sensormanagement.Sensor) error {
		if sensor.SensorID != testSensor.SensorID {
			t.Fatalf("expected sensor id %q, got %q", testSensor.SensorID, sensor.SensorID)
		}
		if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder != "elsys" {
			t.Fatalf("expected sensor profile decoder elsys, got %+v", sensor.SensorProfile)
		}
		return nil
	}

	payload := `{"sensorID":"test-sensor-standalone","sensorProfile":{"decoder":"elsys"}}`
	statusCode, _ := do(t, http.MethodPost, baseUrl+"/api/v0/sensors", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", statusCode)
	}
}

func testCreateSensorDuplicate(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return testSensor, true, nil
	}

	payload := `{"sensorID":"test-sensor-standalone","sensorProfile":{"decoder":"elsys"}}`
	statusCode, _ := do(t, http.MethodPost, baseUrl+"/api/v0/sensors", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", statusCode)
	}
}

func testCreateDevices(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.writer.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		return nil
	}

	mocks.reader.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, bool, error) {
		return types.Device{}, false, nil
	}

	body, contentType := createMultipartFileUpload(t, "fileupload", "devices.csv", csvMock)

	statusCode, _ := do(t, http.MethodPost, baseUrl+"/api/v0/devices", body, map[string]string{"Content-Type": contentType})
	if statusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", statusCode)
	}
}

func testUpdateDevice(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.writer.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		if d.DeviceID != testDevice.DeviceID {
			t.Fatalf("expected device id %q, got %q", testDevice.DeviceID, d.DeviceID)
		}
		return nil
	}

	payload := `{"deviceID":"test-device-1","sensorID":"test-sensor-1","tenant":"default"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/devices/test-device-1", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func testUpdateSensor(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return testSensor, true, nil
	}
	mocks.writer.UpdateFunc = func(ctx context.Context, sensor sensormanagement.Sensor) error {
		if sensor.SensorID != testSensor.SensorID {
			t.Fatalf("expected sensor id %q, got %q", testSensor.SensorID, sensor.SensorID)
		}
		if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder != "enviot" {
			t.Fatalf("expected sensor profile decoder enviot, got %+v", sensor.SensorProfile)
		}
		return nil
	}

	payload := `{"sensorID":"test-sensor-standalone","sensorProfile":{"decoder":"enviot"}}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/sensors/test-sensor-standalone", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func testUpdateSensorNotFound(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return sensormanagement.Sensor{}, false, nil
	}

	payload := `{"sensorID":"missing","sensorProfile":{"decoder":"enviot"}}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/sensors/missing", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testUpdateSensorInternalError(t *testing.T, baseUrl string, mocks sensorManagementMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (sensormanagement.Sensor, bool, error) {
		return testSensor, true, nil
	}
	mocks.writer.UpdateFunc = func(ctx context.Context, sensor sensormanagement.Sensor) error {
		return errors.New("update failed")
	}

	payload := `{"sensorID":"test-sensor-standalone","sensorProfile":{"decoder":"enviot"}}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/sensors/test-sensor-standalone", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testUpdateDeviceNotFound(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 0}, nil
	}

	payload := `{"deviceID":"missing-device","sensorID":"test-sensor-1","tenant":"default"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/devices/missing-device", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testUpdateDeviceInternalError(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.writer.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		return errors.New("write failed")
	}

	payload := `{"deviceID":"test-device-1","sensorID":"test-sensor-1","tenant":"default"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/devices/test-device-1", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testPatchDevice(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.writer.UpdateDeviceFunc = func(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error {
		if deviceID != testDevice.DeviceID {
			t.Fatalf("expected device id %q, got %q", testDevice.DeviceID, deviceID)
		}
		if active == nil || !*active {
			t.Fatalf("expected active=true, got %v", active)
		}
		if interval == nil || *interval != 60 {
			t.Fatalf("expected interval 60, got %v", interval)
		}
		if location == nil || location.Latitude != 62.1 || location.Longitude != 17.2 {
			t.Fatalf("expected parsed location, got %+v", location)
		}
		return nil
	}
	mocks.writer.SetDeviceProfileTypesFunc = func(ctx context.Context, deviceID string, typesMoqParam []types.Lwm2mType) error {
		if len(typesMoqParam) != 1 || typesMoqParam[0].Urn != "urn:test:1" {
			t.Fatalf("expected parsed types, got %+v", typesMoqParam)
		}
		return nil
	}
	mocks.writer.SetSensorProfileFunc = func(ctx context.Context, deviceID string, dp types.SensorProfile) error {
		if dp.Decoder != "profile-a" {
			t.Fatalf("expected profile-a, got %+v", dp)
		}
		return nil
	}

	payload := `{"active":"true","interval":60,"latitude":"62.1","longitude":17.2,"types":["urn:test:1"],"deviceProfile":"profile-a"}`
	statusCode, _ := do(t, http.MethodPatch, baseUrl+"/api/v0/devices/test-device-1", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func testPatchDeviceInvalidField(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}

	payload := `{"active":[true]}`
	statusCode, _ := do(t, http.MethodPatch, baseUrl+"/api/v0/devices/test-device-1", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", statusCode)
	}
}

func testPatchDeviceNotFound(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 0}, nil
	}

	payload := `{}`
	statusCode, _ := do(t, http.MethodPatch, baseUrl+"/api/v0/devices/missing-device", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testPatchDeviceInternalError(t *testing.T, baseUrl string, mocks deviceManagementMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.writer.UpdateDeviceFunc = func(ctx context.Context, deviceID string, active *bool, name, description, environment, source, tenant *string, location *types.Location, interval *int) error {
		return errors.New("write failed")
	}

	payload := `{"active":true}`
	statusCode, _ := do(t, http.MethodPatch, baseUrl+"/api/v0/devices/test-device-1", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
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

var testSensor = sensormanagement.Sensor{
	SensorID: "test-sensor-standalone",
	SensorProfile: &types.SensorProfile{
		Name:     "Elsys",
		Decoder:  "elsys",
		Interval: 60,
	},
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

allow = response {
	response := {
		"tenants": ["default"]
	}
}`
