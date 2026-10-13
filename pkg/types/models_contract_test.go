package types

import (
	"encoding/json"
	"testing"

	"github.com/matryer/is"
)

// DM-003: locks the JSON wire format of the models iot-agent builds
// and sends over HTTP (CreateDevice, CreateSensor). Field names, tags
// and nesting must not change without a compatibility/migration plan
// and consumer-side tests.
func TestDeviceJSONContract(t *testing.T) {
	is := is.New(t)

	d := Device{
		SensorID: "sensor-1",
		DeviceID: "device-1",
		Active:   true,
		Name:     "Device one",
		Location: Location{Latitude: 62.39, Longitude: 17.31},
		Tenant:   "acme",
		Lwm2mTypes: []Lwm2mType{
			{Urn: "urn:oma:lwm2m:ext:3303"},
		},
		SensorProfile: SensorProfile{Name: "elsys", Decoder: "elsys"},
	}

	const golden = `{"sensorID":"sensor-1","deviceID":"device-1","active":true,"name":"Device one","location":{"latitude":62.39,"longitude":17.31},"tenant":"acme","types":[{"urn":"urn:oma:lwm2m:ext:3303","name":""}],"deviceState":{"online":false,"observedAt":"0001-01-01T00:00:00Z"},"sensorProfile":{"name":"elsys","decoder":"elsys","interval":0}}`
	is.Equal(mustMarshal(t, d), golden)

	var decoded Device
	is.NoErr(json.Unmarshal([]byte(golden), &decoded))
	is.Equal(decoded.DeviceID, "device-1")
	is.Equal(decoded.Tenant, "acme")
	is.Equal(decoded.Location.Latitude, 62.39)
	is.Equal(len(decoded.Lwm2mTypes), 1)
	is.Equal(decoded.Lwm2mTypes[0].Urn, "urn:oma:lwm2m:ext:3303")
}

// DM-003: locks the JSON wire format of the sensor input model.
func TestSensorInputModelJSONContract(t *testing.T) {
	is := is.New(t)

	name := "Test sensor"
	s := SensorInputModel{SensorID: "sensor-1", SensorProfileID: "elsys", Name: &name}

	const golden = `{"sensorID":"sensor-1","sensorProfileID":"elsys","name":"Test sensor"}`
	is.Equal(mustMarshal(t, s), golden)

	var decoded SensorInputModel
	is.NoErr(json.Unmarshal([]byte(golden), &decoded))
	is.Equal(decoded.SensorID, "sensor-1")
	is.Equal(decoded.SensorProfileID, "elsys")
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()

	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	return string(b)
}
