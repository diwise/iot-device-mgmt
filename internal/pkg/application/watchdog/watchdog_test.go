package watchdog

import (
	"context"

	"testing"
	"time"

	"github.com/matryer/is"
)

/*
var mu sync.Mutex

	func TestCheckLastObserved(t *testing.T) {
		is, ctx := testSetup(t)
		var devices []types.Device
		err := json.Unmarshal([]byte(devicesJson), &devices)
		is.NoErr(err)

		pub := []string{}

		m := &messaging.MsgContextMock{
			PublishOnTopicFunc: func(ctx context.Context, message messaging.TopicMessage) error {
				mu.Lock()
				defer mu.Unlock()
				var msg DeviceNotObserved
				json.Unmarshal(msg.Body(), &msg)
				pub = append(pub, msg.DeviceID)
				return nil
			},
		}

		r := &devicemanagement.DeviceRepositoryMock{
			GetOnlineDevicesFunc: func(ctx context.Context, offset, limit int, tenants []string) (repositories.Collection[types.Device], error) {
				return repositories.Collection[types.Device]{
					Data:       devices,
					Offset:     0,
					Limit:      10,
					Count:      uint64(len(devices)),
					TotalCount: uint64(len(devices)),
				}, nil
			},
			GetTenantsFunc: func(ctx context.Context) []string {
				return []string{"default"}
			},
			GetByDeviceIDFunc: func(ctx context.Context, deviceID string, tenants []string) (types.Device, error) {
				for _, d := range devices {
					if d.DeviceID == deviceID {
						return d, nil
					}
				}
				return types.Device{}, fmt.Errorf("device not found")
			},
		}

		lw := lastObservedWatcher{
			deviceRepository: r,
			messenger:        m,
			running:          false,
			interval:         1 * time.Second,
		}

		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		go lw.Watch(ctx)

		for len(pub) != len(devices) {
		}
	}
*/
func TestCheckLastObservedIsAfter(t *testing.T) {
	is, ctx := testSetup(t)

	observed, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	is.NoErr(err)
	now, err := time.Parse(time.RFC3339, "2006-01-02T15:03:54Z")
	is.NoErr(err)

	is.True(checkLastObservedIsAfter(ctx, observed, now, 10))

	now = now.Add(30 * time.Second)
	is.True(!checkLastObservedIsAfter(ctx, observed, now, 10))
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
        "tenant": "default",
        
        "name": "UltraSonic01",
        "description": "",
        "location": {
            "latitude": 0,
            "longitude": 0
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
        "tenant": "default",
        "name": "mcg-ers-co2-01",
        "description": "Masarinkontoret",
        "location": {
            "latitude": 0,
            "longitude": 0
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
