package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/diwise/iot-device-mgmt/internal/application"
	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	alarmquery "github.com/diwise/iot-device-mgmt/internal/application/alarms/query"
	"github.com/diwise/iot-device-mgmt/internal/application/devices"
	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devices/query"
	"github.com/diwise/iot-device-mgmt/internal/application/sensors"
	sensorquery "github.com/diwise/iot-device-mgmt/internal/application/sensors/query"
	"github.com/diwise/messaging-golang/pkg/messaging"

	"github.com/diwise/iot-device-mgmt/pkg/types"
)

func TestApi(t *testing.T) {
	ctx := t.Context()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	msgMock := &messaging.MsgContextMock{}
	mocks := newDeviceMocks()
	sensorMocks := newSensorMocks()

	dm := devices.New(mocks.reader, mocks.writer, mocks.statusWriter, mocks.profiles, msgMock, &devices.Config{})
	sm := sensors.New(sensorMocks.reader, sensorMocks.writer)
	as := alarms.AlarmAPIServiceMock{}

	app := application.New(dm, sm, &as, true)

	mux := http.NewServeMux()
	RegisterHandlers(ctx, mux, policies, app)

	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("GET /devices", func(t *testing.T) {
		testQueryDevices(t, server.URL, mocks)
	})

	t.Run("GET /sensors", func(t *testing.T) {
		testQuerySensors(t, server.URL, sensorMocks)
	})

	t.Run("GET /sensors with decoder and types filters", func(t *testing.T) {
		testQuerySensorsWithFilters(t, server.URL, sensorMocks)
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
		testGetDeviceProfiles(t, app.DeviceService())
	})

	t.Run("GET /admin/deviceprofiles/missing", func(t *testing.T) {
		testGetDeviceProfilesNotFound(t, app.DeviceService())
	})

	t.Run("GET /admin/deviceprofiles internal failure", func(t *testing.T) {
		testGetDeviceProfilesInternalError(t, app.DeviceService())
	})

	t.Run("GET /admin/lwm2mtypes", func(t *testing.T) {
		testGetLwm2mTypes(t, app.DeviceService())
	})

	t.Run("GET /admin/lwm2mtypes/missing", func(t *testing.T) {
		testGetLwm2mTypesNotFound(t, app.DeviceService())
	})

	t.Run("GET /admin/lwm2mtypes internal failure", func(t *testing.T) {
		testGetLwm2mTypesInternalError(t, app.DeviceService())
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

	t.Run("PUT /devices/test-device-1", func(t *testing.T) {
		testUpdateDevice(t, server.URL, mocks)
	})

	t.Run("PUT /devices/test-device-1/sensor", func(t *testing.T) {
		testAttachSensorToDevice(t, server.URL, mocks)
	})

	t.Run("PUT /devices/test-device-1/sensor conflict", func(t *testing.T) {
		testAttachSensorToDeviceConflict(t, server.URL, mocks)
	})

	t.Run("DELETE /devices/test-device-1/sensor", func(t *testing.T) {
		testDetachSensorFromDevice(t, server.URL, mocks)
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
	devices.DeviceAPIService
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

type deviceMocks struct {
	reader       *devices.DeviceReaderMock
	writer       *devices.DeviceWriterMock
	statusWriter *devices.DeviceStatusWriterMock
	profiles     *devices.DeviceProfileStoreMock
}

type sensorReaderMock struct {
	QueryFunc            func(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error)
	GetFunc              func(ctx context.Context, sensorID string) (types.Sensor, bool, error)
	GetSensorProfileFunc func(ctx context.Context, profileID string) (types.SensorProfile, bool, error)
}

func (m *sensorReaderMock) QuerySensors(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
	if m.QueryFunc == nil {
		panic("sensorReaderMock.QueryFunc is nil")
	}
	return m.QueryFunc(ctx, query)
}

func (m *sensorReaderMock) GetSensor(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
	if m.GetFunc == nil {
		panic("sensorReaderMock.GetFunc is nil")
	}
	return m.GetFunc(ctx, sensorID)
}

func (m *sensorReaderMock) GetSensorProfile(ctx context.Context, profileID string) (types.SensorProfile, bool, error) {
	if m.GetSensorProfileFunc == nil {
		panic("sensorReaderMock.GetSensorProfileFunc is nil")
	}
	return m.GetSensorProfileFunc(ctx, profileID)
}

type sensorWriterMock struct {
	CreateFunc func(ctx context.Context, sensor types.Sensor) error
	UpdateFunc func(ctx context.Context, sensor types.Sensor) error
}

func (m *sensorWriterMock) CreateSensor(ctx context.Context, sensor types.Sensor) error {
	if m.CreateFunc == nil {
		panic("sensorWriterMock.CreateFunc is nil")
	}
	return m.CreateFunc(ctx, sensor)
}

func (m *sensorWriterMock) UpdateSensor(ctx context.Context, sensor types.Sensor) error {
	if m.UpdateFunc == nil {
		panic("sensorWriterMock.UpdateFunc is nil")
	}
	return m.UpdateFunc(ctx, sensor)
}

type sensorMocks struct {
	reader *sensorReaderMock
	writer *sensorWriterMock
}

func newSensorMocks() sensorMocks {
	return sensorMocks{
		reader: &sensorReaderMock{},
		writer: &sensorWriterMock{},
	}
}

func newNoopSensorAPIService() sensors.SensorAPIService {
	mocks := sensorMocks{
		reader: &sensorReaderMock{
			QueryFunc: func(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
				return types.Collection[types.Sensor]{}, nil
			},
			GetFunc: func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
				return types.Sensor{}, false, nil
			},
		},
		writer: &sensorWriterMock{
			CreateFunc: func(ctx context.Context, sensor types.Sensor) error { return nil },
			UpdateFunc: func(ctx context.Context, sensor types.Sensor) error { return nil },
		},
	}

	return sensors.New(mocks.reader, mocks.writer)
}

func newDeviceMocks() deviceMocks {
	return deviceMocks{
		reader:       &devices.DeviceReaderMock{},
		writer:       &devices.DeviceWriterMock{},
		statusWriter: &devices.DeviceStatusWriterMock{},
		profiles:     &devices.DeviceProfileStoreMock{},
	}
}

func testQueryDevices(t *testing.T, baseUrl string, mocks deviceMocks) {
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

func testQuerySensors(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
		return types.Collection[types.Sensor]{
			Data:       []types.Sensor{testSensor},
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

func testQuerySensorsWithFilters(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
		if query.ProfileName != "Elsys" {
			t.Fatalf("expected profileName filter Elsys, got %q", query.ProfileName)
		}
		if len(query.Types) != 2 || query.Types[0] != "urn:oma:lwm2m:ext:3303" || query.Types[1] != "urn:oma:lwm2m:ext:3304" {
			t.Fatalf("expected types filter, got %+v", query.Types)
		}

		return types.Collection[types.Sensor]{
			Data:       []types.Sensor{testSensor},
			Count:      1,
			Offset:     0,
			Limit:      10,
			TotalCount: 1,
		}, nil
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors?profileName=Elsys&types=urn:oma:lwm2m:ext:3303&types=urn:oma:lwm2m:ext:3304", nil)
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func testQuerySensorsWithInvalidLimit(t *testing.T, baseUrl string) {
	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors?limit=invalid", nil)
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", statusCode)
	}
}

func testQuerySensorsInternalError(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query sensorquery.Sensors) (types.Collection[types.Sensor], error) {
		return types.Collection[types.Sensor]{}, errors.New("query failed")
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testGetSensor(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
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

func testGetSensorNotFound(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{}, false, nil
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/sensors/missing", nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testGetSensorInternalError(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{}, false, errors.New("lookup failed")
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

func testQueryDevicesBySensorID(t *testing.T, baseUrl string, mocks deviceMocks) {
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

func testGetDevice(t *testing.T, baseUrl string, mocks deviceMocks) {
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

func testDeviceStatus(t *testing.T, baseUrl string, mocks deviceMocks) {
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

func testDeviceAlarms(t *testing.T, baseUrl string, mocks deviceMocks) {
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

func testDeviceMeasurements(t *testing.T, baseUrl string, mocks deviceMocks) {
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

func testGetAlarms(t *testing.T, baseUrl string, service *alarms.AlarmAPIServiceMock) {
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

func testGetAlarmsInternalError(t *testing.T, baseUrl string, service *alarms.AlarmAPIServiceMock) {
	service.AlarmsFunc = func(ctx context.Context, query alarmquery.Alarms) (types.Collection[types.Alarms], error) {
		return types.Collection[types.Alarms]{}, errors.New("storage failure")
	}

	statusCode, _ := do(t, http.MethodGet, baseUrl+"/api/v0/alarms", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testGetDeviceProfiles(t *testing.T, dm devices.DeviceAPIService) {
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

	app := application.New(service, newNoopSensorAPIService(), &alarms.AlarmAPIServiceMock{}, true)

	mux := http.NewServeMux()
	if err := RegisterHandlers(t.Context(), mux, policies, app); err != nil {
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

func testGetDeviceProfilesNotFound(t *testing.T, dm devices.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		profilesFunc: func(ctx context.Context, name ...string) (types.Collection[types.SensorProfile], error) {
			return types.Collection[types.SensorProfile]{}, devices.ErrDeviceProfileNotFound
		},
	}

	mux := http.NewServeMux()
	app := application.New(service, newNoopSensorAPIService(), &alarms.AlarmAPIServiceMock{}, true)
	if err := RegisterHandlers(t.Context(), mux, policies, app); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/deviceprofiles/missing", nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testGetDeviceProfilesInternalError(t *testing.T, dm devices.DeviceAPIService) {
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
	app := application.New(service, newNoopSensorAPIService(), &alarms.AlarmAPIServiceMock{}, true)
	if err := RegisterHandlers(t.Context(), mux, policies, app); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/deviceprofiles", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testGetLwm2mTypes(t *testing.T, dm devices.DeviceAPIService) {
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
	app := application.New(service, newNoopSensorAPIService(), &alarms.AlarmAPIServiceMock{}, true)
	if err := RegisterHandlers(t.Context(), mux, policies, app); err != nil {
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

func testGetLwm2mTypesNotFound(t *testing.T, dm devices.DeviceAPIService) {
	t.Helper()

	policies := io.NopCloser(strings.NewReader(policiesMock))
	defer policies.Close()

	service := adminAPIService{
		DeviceAPIService: dm,
		lwm2mTypesFunc: func(ctx context.Context, urn ...string) (types.Collection[types.Lwm2mType], error) {
			return types.Collection[types.Lwm2mType]{}, devices.ErrDeviceProfileNotFound
		},
	}

	mux := http.NewServeMux()
	app := application.New(service, newNoopSensorAPIService(), &alarms.AlarmAPIServiceMock{}, true)
	if err := RegisterHandlers(t.Context(), mux, policies, app); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/lwm2mtypes/missing", nil)
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testGetLwm2mTypesInternalError(t *testing.T, dm devices.DeviceAPIService) {
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
	app := application.New(service, newNoopSensorAPIService(), &alarms.AlarmAPIServiceMock{}, true)
	if err := RegisterHandlers(t.Context(), mux, policies, app); err != nil {
		t.Fatalf("failed to register handlers: %v", err)
	}

	server := httptest.NewServer(mux)
	defer server.Close()

	statusCode, _ := do(t, http.MethodGet, server.URL+"/api/v0/admin/lwm2mtypes", nil)
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testCreateDevice(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.writer.CreateOrUpdateDeviceFunc = func(ctx context.Context, d types.Device) error {
		return nil
	}

	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{
			Data: []types.Device{},
		}, nil
	}
	mocks.reader.GetSensorFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{SensorID: sensorID, SensorProfile: &types.SensorProfile{Decoder: "elsys"}}, true, nil
	}
	mocks.reader.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, bool, error) {
		return types.Device{}, false, nil
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

func testCreateSensor(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{}, false, nil
	}
	mocks.writer.CreateFunc = func(ctx context.Context, sensor types.Sensor) error {
		if sensor.SensorID != testSensor.SensorID {
			t.Fatalf("expected sensor id %q, got %q", testSensor.SensorID, sensor.SensorID)
		}
		if sensor.Name == nil || *sensor.Name != "Outdoor sensor" {
			t.Fatalf("expected sensor name Outdoor sensor, got %+v", sensor.Name)
		}
		if sensor.Location == nil || sensor.Location.Latitude != 62.3901 || sensor.Location.Longitude != 17.3069 {
			t.Fatalf("expected sensor location {62.3901 17.3069}, got %+v", sensor.Location)
		}
		if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder != "elsys" {
			t.Fatalf("expected sensor profile decoder elsys, got %+v", sensor.SensorProfile)
		}
		return nil
	}

	payload := `{"sensorID":"test-sensor-standalone","name":"Outdoor sensor","location":{"latitude":62.3901,"longitude":17.3069},"sensorProfile":{"decoder":"elsys"}}`
	statusCode, _ := do(t, http.MethodPost, baseUrl+"/api/v0/sensors", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", statusCode)
	}
}

func testCreateSensorDuplicate(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return testSensor, true, nil
	}

	payload := `{"sensorID":"test-sensor-standalone","sensorProfile":{"decoder":"elsys"}}`
	statusCode, _ := do(t, http.MethodPost, baseUrl+"/api/v0/sensors", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", statusCode)
	}
}

func testUpdateDevice(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.reader.GetSensorFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{SensorID: sensorID, SensorProfile: &types.SensorProfile{Decoder: "elsys"}}, true, nil
	}
	mocks.reader.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, bool, error) {
		return testDevice, true, nil
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

func testUpdateSensor(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return testSensor, true, nil
	}
	mocks.reader.GetSensorProfileFunc = func(ctx context.Context, profileID string) (types.SensorProfile, bool, error) {
		if profileID != "enviot" {
			t.Fatalf("expected profile id enviot, got %q", profileID)
		}
		return types.SensorProfile{
			Name:     "Enviot",
			Decoder:  "enviot",
			Interval: 60,
		}, true, nil
	}
	mocks.writer.UpdateFunc = func(ctx context.Context, sensor types.Sensor) error {
		if sensor.SensorID != testSensor.SensorID {
			t.Fatalf("expected sensor id %q, got %q", testSensor.SensorID, sensor.SensorID)
		}
		if sensor.SensorProfile == nil || sensor.SensorProfile.Decoder != "enviot" {
			t.Fatalf("expected sensor profile decoder enviot, got %+v", sensor.SensorProfile)
		}
		return nil
	}

	payload := `{"sensorID":"test-sensor-standalone","sensorProfileID":"enviot"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/sensors/test-sensor-standalone", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func testUpdateSensorNotFound(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{}, false, nil
	}
	mocks.reader.GetSensorProfileFunc = func(ctx context.Context, profileID string) (types.SensorProfile, bool, error) {
		if profileID != "enviot" {
			t.Fatalf("expected profile id enviot, got %q", profileID)
		}
		return types.SensorProfile{
			Name:     "Enviot",
			Decoder:  "enviot",
			Interval: 60,
		}, true, nil
	}

	payload := `{"sensorID":"missing","sensorProfileID":"enviot"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/sensors/missing", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testUpdateSensorInternalError(t *testing.T, baseUrl string, mocks sensorMocks) {
	mocks.reader.GetFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return testSensor, true, nil
	}
	mocks.reader.GetSensorProfileFunc = func(ctx context.Context, profileID string) (types.SensorProfile, bool, error) {
		if profileID != "enviot" {
			t.Fatalf("expected profile id enviot, got %q", profileID)
		}
		return types.SensorProfile{
			Name:     "Enviot",
			Decoder:  "enviot",
			Interval: 60,
		}, true, nil
	}
	mocks.writer.UpdateFunc = func(ctx context.Context, sensor types.Sensor) error {
		return errors.New("update failed")
	}

	payload := `{"sensorID":"test-sensor-standalone","sensorProfileID":"enviot"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/sensors/test-sensor-standalone", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", statusCode)
	}
}

func testUpdateDeviceNotFound(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 0}, nil
	}

	payload := `{"deviceID":"missing-device","sensorID":"test-sensor-1","tenant":"default"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/devices/missing-device", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testUpdateDeviceInternalError(t *testing.T, baseUrl string, mocks deviceMocks) {
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

func testPatchDevice(t *testing.T, baseUrl string, mocks deviceMocks) {
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

	payload := `{"active":"true","interval":60,"latitude":"62.1","longitude":17.2,"types":["urn:test:1"]}`
	statusCode, _ := do(t, http.MethodPatch, baseUrl+"/api/v0/devices/test-device-1", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func testAttachSensorToDevice(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.reader.GetSensorFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{SensorID: sensorID, SensorProfile: &types.SensorProfile{Decoder: "elsys"}}, true, nil
	}
	mocks.reader.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, bool, error) {
		return types.Device{}, false, nil
	}
	mocks.writer.AssignSensorFunc = func(ctx context.Context, deviceID, sensorID string) error {
		if deviceID != testDevice.DeviceID {
			t.Fatalf("expected device id %q, got %q", testDevice.DeviceID, deviceID)
		}
		if sensorID != "test-sensor-standalone" {
			t.Fatalf("expected sensor id %q, got %q", "test-sensor-standalone", sensorID)
		}
		return nil
	}

	payload := `{"sensorID":"test-sensor-standalone"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/devices/test-device-1/sensor", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", statusCode)
	}
}

func testAttachSensorToDeviceConflict(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.reader.GetSensorFunc = func(ctx context.Context, sensorID string) (types.Sensor, bool, error) {
		return types.Sensor{SensorID: sensorID}, true, nil
	}
	mocks.reader.GetDeviceBySensorIDFunc = func(ctx context.Context, sensorID string) (types.Device, bool, error) {
		return types.Device{}, false, nil
	}

	payload := `{"sensorID":"test-sensor-standalone"}`
	statusCode, _ := do(t, http.MethodPut, baseUrl+"/api/v0/devices/test-device-1/sensor", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusConflict {
		t.Fatalf("expected status 409, got %d", statusCode)
	}
}

func testDetachSensorFromDevice(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}
	mocks.writer.UnassignSensorFunc = func(ctx context.Context, deviceID string) error {
		if deviceID != testDevice.DeviceID {
			t.Fatalf("expected device id %q, got %q", testDevice.DeviceID, deviceID)
		}
		return nil
	}

	statusCode, _ := do(t, http.MethodDelete, baseUrl+"/api/v0/devices/test-device-1/sensor", nil)
	if statusCode != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", statusCode)
	}
}

func testPatchDeviceInvalidField(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 1, Data: []types.Device{testDevice}}, nil
	}

	payload := `{"active":[true]}`
	statusCode, _ := do(t, http.MethodPatch, baseUrl+"/api/v0/devices/test-device-1", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", statusCode)
	}
}

func testPatchDeviceNotFound(t *testing.T, baseUrl string, mocks deviceMocks) {
	mocks.reader.QueryFunc = func(ctx context.Context, query dmquery.Devices) (types.Collection[types.Device], error) {
		return types.Collection[types.Device]{Count: 0}, nil
	}

	payload := `{}`
	statusCode, _ := do(t, http.MethodPatch, baseUrl+"/api/v0/devices/missing-device", strings.NewReader(payload), map[string]string{"Content-Type": "application/json"})
	if statusCode != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", statusCode)
	}
}

func testPatchDeviceInternalError(t *testing.T, baseUrl string, mocks deviceMocks) {
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

var testSensor = types.Sensor{
	SensorID: "test-sensor-standalone",
	SensorProfile: &types.SensorProfile{
		Name:     "Elsys",
		Decoder:  "elsys",
		Interval: 60,
	},
}

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
                "devices.update",
                "sensors.create",
                "sensors.read",
                "sensors.update"
            ]
        }
	}
}`
