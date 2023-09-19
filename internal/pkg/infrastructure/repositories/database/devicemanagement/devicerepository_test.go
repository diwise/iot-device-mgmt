package devicemanagement

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"

	"github.com/matryer/is"
)

func TestGetDevices(t *testing.T) {
	is, ctx, r := testSetupDeviceRepository(t)

	r.Save(ctx, createDevice(1, "default"))
	r.Save(ctx, createDevice(2, "default"))
	r.Save(ctx, createDevice(3, "default"))
	r.Save(ctx, createDevice(4, "test"))
	r.Save(ctx, createDevice(5, "test"))
	r.Save(ctx, createDevice(6, "secret"))

	defaultTenantDevices, err := r.GetDevices(ctx, "default")
	is.NoErr(err)
	is.Equal(3, len(defaultTenantDevices))

	testTenantDevices, err := r.GetDevices(ctx, "test")
	is.NoErr(err)
	is.Equal(2, len(testTenantDevices))

	allTenantDevices, err := r.GetDevices(ctx, "default", "test", "secret")
	is.NoErr(err)
	is.Equal(6, len(allTenantDevices))

	onlineDevices, err := r.GetOnlineDevices(ctx)
	is.NoErr(err)
	is.Equal(0, len(onlineDevices))
}

func TestSaveAndGet(t *testing.T) {
	is, ctx, r := testSetupDeviceRepository(t)

	err := r.Save(ctx, createDevice(1, "default"))
	is.NoErr(err)

	err = r.Save(ctx, createDevice(2, "default"))
	is.NoErr(err)

	fromDb, err := r.GetDeviceByDeviceID(ctx, "device-1")
	is.NoErr(err)
	is.Equal("device-1", fromDb.DeviceID)
	is.Equal("urn:3340", fromDb.Lwm2mTypes[0].Urn)

	fromDb2, err := r.GetDeviceByDeviceID(ctx, "device-2", "default")
	is.NoErr(err)
	is.Equal("device-2", fromDb2.DeviceID)
	is.Equal("tag-01", fromDb2.Tags[0].Name)

	is.Equal(fromDb.Tags[0].ID, fromDb2.Tags[0].ID)
}

func TestUpdateDeviceStatus(t *testing.T) {
	is, ctx, r := testSetupDeviceRepository(t)

	err := r.Save(ctx, createDevice(1, "default"))
	is.NoErr(err)

	newStatus := DeviceStatus{
		BatteryLevel: 50,
		LastObserved: time.Now(),
	}

	err = r.UpdateDeviceStatus(ctx, "device-1", newStatus)
	is.NoErr(err)

	fromDb, err := r.GetDeviceByDeviceID(ctx, "device-1")
	is.NoErr(err)

	is.Equal(50, fromDb.DeviceStatus.BatteryLevel)
}

func TestUpdateDeviceState(t *testing.T) {
	is, ctx, r := testSetupDeviceRepository(t)

	err := r.Save(ctx, createDevice(1, "default"))
	is.NoErr(err)

	newState := DeviceState{
		State:      DeviceStateError,
		ObservedAt: time.Now(),
	}

	err = r.UpdateDeviceState(ctx, "device-1", newState)
	is.NoErr(err)

	fromDb, err := r.GetDeviceByDeviceID(ctx, "device-1")
	is.NoErr(err)

	is.Equal(DeviceStateError, fromDb.DeviceState.State)
}

func TestSeed(t *testing.T) {
	is, ctx, r := testSetupDeviceRepository(t)

	csv := bytes.NewBuffer([]byte(csvMock))

	err := r.Seed(ctx, csv)
	is.NoErr(err)

	devices, err := r.GetDevices(ctx, "_default")
	is.NoErr(err)
	is.Equal(2, len(devices))
	is.Equal("_default", devices[0].Tenant.Name)

	devices, err = r.GetDevices(ctx, "_test")
	is.NoErr(err)
	is.Equal(2, len(devices))
	is.Equal("_test", devices[0].Tenant.Name)

	device, err := r.GetDeviceByDeviceID(ctx, "intern-1234")
	is.NoErr(err)
	is.Equal("urn:oma:lwm2m:ext:3304", device.Lwm2mTypes[1].Urn)
	is.Equal(60, device.DeviceProfile.Interval)
	is.Equal("källa", device.Source)
}

func TestAlarms(t *testing.T) {
	is, ctx, r := testSetupDeviceRepository(t)

	err := r.Save(ctx, createDevice(50, "default"))
	is.NoErr(err)

	d, err := r.GetDeviceByDeviceID(ctx, "device-50")
	is.NoErr(err)
	is.Equal(0, len(d.Alarms))

	err = r.AddAlarm(ctx, "device-50", 1, 1, time.Now())
	is.NoErr(err)

	d, err = r.GetDeviceByDeviceID(ctx, "device-50")
	is.NoErr(err)
	is.Equal(1, len(d.Alarms))

	deviceID, err := r.RemoveAlarmByID(ctx, 1)
	is.NoErr(err)
	is.Equal("device-50", deviceID)

	d, err = r.GetDeviceByDeviceID(ctx, "device-50")
	is.NoErr(err)
	is.Equal(0, len(d.Alarms))
}

func testSetupDeviceRepository(t *testing.T) (*is.I, context.Context, DeviceRepository) {
	is, ctx, conn := setup(t)

	r, _ := NewDeviceRepository(conn)

	return is, ctx, r
}

func setup(t *testing.T) (*is.I, context.Context, ConnectorFunc) {
	is := is.New(t)
	ctx := context.Background()
	conn := NewSQLiteConnector(ctx)

	return is, ctx, conn
}

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

const csvMock string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant;interval;source
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;_default;60;source
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec;name-a81758fffe051d00;desc-a81758fffe051d00;true;_default;60;source
1234;intern-1234;0.0;0.0;air;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3304;enviot;name-1234;desc-1234;true;_test;60;källa
5678;intern-5678;0.0;0.0;soil;urn:oma:lwm2m:ext:3303;enviot;name-5678;desc-5678;true;_test;60; 
`
