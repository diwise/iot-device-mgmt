package devicemanagement

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
	"time"

	"testing"

	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	. "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestGetDevices(t *testing.T) {
	is, ctx, repo := testSetupAndSeed(t)

	collection, err := repo.GetDevices(ctx, 0, 5, []string{"default", "_default", "_test"})
	is.NoErr(err)

	is.True(len(collection.Data) > 0)
}

func TestUpdateDeviceState(t *testing.T) {
	is, ctx, repo := testSetupAndSeed(t)

	state := rand.Intn(100)

	err := repo.UpdateDeviceState(ctx, "intern-5679", "default", DeviceState{
		Online:     true,
		State:      state,
		ObservedAt: time.Now(),
	})
	is.NoErr(err)

	device, err := repo.GetDeviceByDeviceID(ctx, "intern-5679", []string{"default"})
	is.NoErr(err)

	is.Equal(state, device.DeviceState.State)
	is.True(device.DeviceState.Online)
}

func TestUpdateDeviceStatus(t *testing.T) {
	is, ctx, repo := testSetupAndSeed(t)

	bat := rand.Intn(100)

	err := repo.UpdateDeviceStatus(ctx, "intern-5679", "default", DeviceStatus{
		BatteryLevel: bat,
		ObservedAt:   time.Now(),
	})
	is.NoErr(err)

	device, err := repo.GetDeviceByDeviceID(ctx, "intern-5679", []string{"default"})
	is.NoErr(err)

	is.Equal(bat, device.DeviceStatus.BatteryLevel)
}

func TestGetDeviceByDeviceID(t *testing.T) {
	is, ctx, repo := testSetupAndSeed(t)

	device, err := repo.GetDeviceByDeviceID(ctx, "intern-5679", []string{"default"})
	is.NoErr(err)

	is.Equal("intern-5679", device.DeviceID)
	is.Equal("axsensor", device.DeviceProfile.Decoder)
}

func TestGetDeviceBySensorID(t *testing.T) {
	is, ctx, repo := testSetupAndSeed(t)
	device, err := repo.GetDeviceBySensorID(ctx, "5679", []string{"default"})
	is.NoErr(err)

	is.Equal("intern-5679", device.DeviceID)
	is.Equal("axsensor", device.DeviceProfile.Decoder)
}

func TestDeviceNotFound(t *testing.T) {
	is, ctx, repo := testSetupAndSeed(t)
	_, err := repo.GetDeviceBySensorID(ctx, uuid.NewString(), []string{"default"})
	is.True(errors.Is(err, ErrDeviceNotFound))
}

func TestSeed(t *testing.T) {
	is, ctx, repo := testSetup(t)
	r := bytes.NewBuffer([]byte(csvMock))

	err := repo.Seed(ctx, r, []string{"default"})
	is.NoErr(err)
}

func TestAlarms(t *testing.T) {

}

func testSetupAndSeed(t *testing.T) (*is.I, context.Context, DeviceRepository) {
	is, ctx, repo := testSetup(t)
	r := bytes.NewBuffer([]byte(csvMock))

	err := repo.Seed(ctx, r, []string{"default"})
	is.NoErr(err)

	return is, ctx, repo
}

func testSetup(t *testing.T) (*is.I, context.Context, DeviceRepository) {
	is := is.New(t)
	ctx := context.Background()
	config := jsonstore.NewConfig(
		"localhost",
		"postgres",
		"password",
		"5432",
		"postgres",
		"disable",
	)

	p, _ := jsonstore.NewPool(ctx, config)
	repo, err := NewRepository(ctx, p)
	is.NoErr(err)

	return is, ctx, repo
}

/*
	func createDevice(n int, tenant string) *Device {
		return &Device{
			Active:   true,
			SensorID: fmt.Sprintf("sensor-%d", n),
			DeviceID: fmt.Sprintf("device-%d", n),
			Tenant: Tenant{
				Name: tenant,
			},
			Name:        "name",
			Description: "description",
			Location: Location{
				Latitude:  16.0,
				Longitude: 63.0,
				Altitude:  0.0,
			},
			Tags: []Tag{{Name: "tag-01"}},
			Lwm2mTypes: []Lwm2mType{
				{Urn: "urn:3340"},
			},
			DeviceProfile: DeviceProfile{
				Name:    "deviceProfile",
				Decoder: "decoder",
			},
			DeviceStatus: DeviceStatus{
				BatteryLevel: 100,
				LastObserved: time.Now(),
			},
			DeviceState: DeviceState{
				Online: false,
				State:  DeviceStateOK,
			},
		}
	}
*/
const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;_default;60;source
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;_default;60;source
1234;intern-1234;0.0;0.0;air;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3304;enviot;name-1234;desc-1234;true;_test;60;källa
5678;intern-5678;0.0;0.0;soil;urn:oma:lwm2m:ext:3303;enviot;name-5678;desc-5678;true;_test;60;
5679;intern-5679;0.0;0.0;;urn:oma:lwm2m:ext:3330,urn:oma:lwm2m:ext:3;axsensor;AXsensor;Mäter nivå i avlopp;true;default;0;
`

const internCsv string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
5678;intern-5678;62.0;17.0;soil;urn:oma:lwm2m:ext:3302;enviot;name-5678;desc-5678;false;_test;60; 
`

//test
