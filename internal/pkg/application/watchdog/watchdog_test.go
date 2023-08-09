package watchdog

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestCheckLastObserved(t *testing.T) {
	is, ctx := testSetup(t)
	var devices []devicemanagement.Device
	err := json.Unmarshal([]byte(devicesJson), &devices)
	is.NoErr(err)

	m := &messaging.MsgContextMock{}
	r := &devicemanagement.DeviceRepositoryMock{
		GetOnlineDevicesFunc: func(ctx context.Context, tenants ...string) ([]devicemanagement.Device, error) {
			devices[0].DeviceStatus.LastObserved = time.Now()
			return devices, nil
		},
	}
	lw := lastObservedWatcher{
		deviceRepository: r,
		messenger:        m,
	}

	checked, err := lw.checkLastObserved(ctx, zerolog.Logger{})
	is.NoErr(err)

	is.Equal(1, len(checked))
}

func TestCheckLastObservedIsAfter(t *testing.T) {
	is, _ := testSetup(t)

	observed, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	is.NoErr(err)
	now, err := time.Parse(time.RFC3339, "2006-01-02T15:03:54Z")
	is.NoErr(err)

	is.True(checkLastObservedIsAfter(zerolog.Logger{}, observed, now, 10))

	now = now.Add(30 * time.Second)
	is.True(!checkLastObservedIsAfter(zerolog.Logger{}, observed, now, 10))
}

func TestBatteryLevelChangedPublish(t *testing.T) {
	is, ctx := testSetup(t)
	var devices []devicemanagement.Device
	err := json.Unmarshal([]byte(devicesJson), &devices)
	is.NoErr(err)

	var msg messaging.TopicMessage

	m := &messaging.MsgContextMock{
		PublishOnTopicFunc: func(ctx context.Context, message messaging.TopicMessage) error {
			msg = message
			return nil
		},
	}
	r := &devicemanagement.DeviceRepositoryMock{
		GetOnlineDevicesFunc: func(ctx context.Context, tenants ...string) ([]devicemanagement.Device, error) {
			return devices, nil
		},
		GetDeviceByDeviceIDFunc: func(ctx context.Context, deviceID string, tenants ...string) (devicemanagement.Device, error) {
			for _, d := range devices {
				if d.DeviceID == deviceID {
					return d, nil
				}
			}
			return devicemanagement.Device{}, fmt.Errorf("device not found")
		},
	}

	bw := &batteryLevelWatcher{
		deviceRepository: r,
		batteryLevels:    make(map[string]int),
		messenger:        m,
	}

	err = bw.publish(ctx, devices[0].DeviceID)
	is.NoErr(err)

	is.Equal("watchdog.batteryLevelChanged", msg.TopicName())
}

func TestCheckBatteryLevel(t *testing.T) {
	is, ctx := testSetup(t)

	var devices []devicemanagement.Device
	err := json.Unmarshal([]byte(devicesJson), &devices)
	is.NoErr(err)

	m := &messaging.MsgContextMock{}
	r := &devicemanagement.DeviceRepositoryMock{
		GetOnlineDevicesFunc: func(ctx context.Context, tenants ...string) ([]devicemanagement.Device, error) {
			return devices, nil
		},
	}

	bw := &batteryLevelWatcher{
		deviceRepository: r,
		batteryLevels:    make(map[string]int),
		messenger:        m,
	}

	checked, err := bw.checkBatteryLevels(ctx)
	is.NoErr(err)
	is.Equal(0, len(checked))

	devices[0].DeviceStatus.BatteryLevel = 5

	checked, err = bw.checkBatteryLevels(ctx)
	is.NoErr(err)

	is.Equal(1, len(checked))
}

func testSetup(t *testing.T) (*is.I, context.Context) {

	return is.New(t), context.Background()
}

const devicesJson string = `
[
    {
        "active": true,
        "sensorID": "01",
        "deviceID": "device:01",
        "tenant": {
            "name": "default"
        },
        "name": "UltraSonic01",
        "description": "",
        "location": {
            "latitude": 0,
            "longitude": 0,
            "altitude": 0
        },
        "environment": "",
        "types": [
            {
                "urn": "urn:oma:lwm2m:ext:3303"
            }
        ],
        "tags": [],
        "deviceProfile": {
            "name": "elsys",
            "decoder": "elsys",
            "interval": 3600
        },
        "deviceStatus": {
            "batteryLevel": 20,
            "lastObservedAt": "2023-01-01T00:00:00Z"
        },
        "deviceState": {
            "online": true,
            "state": 0,
            "observedAt": "2023-01-01T00:00:00Z"
        }
    },
    {
        "active": true,
        "sensorID": "02",
        "deviceID": "device-02",
        "tenant": {
            "name": "default"
        },
        "name": "mcg-ers-co2-01",
        "description": "Masarinkontoret",
        "location": {
            "latitude": 0,
            "longitude": 0,
            "altitude": 0
        },
        "environment": "indoors",
        "types": [
            {
                "urn": "urn:oma:lwm2m:ext:3303"
            }
        ],
        "tags": [],
        "deviceProfile": {
            "name": "elsys",
            "decoder": "elsys",
            "interval": 3600
        },
        "deviceStatus": {
            "batteryLevel": 10,
            "lastObservedAt": "2023-01-01T00:00:00Z"
        },
        "deviceState": {
            "online": true,
            "state": 0,
            "observedAt": "2023-01-01T00:00:00Z"
        }
    }
]

`
