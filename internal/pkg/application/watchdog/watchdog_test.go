package watchdog

import (
	"context"

	"testing"
	"time"

	"github.com/matryer/is"
)

func TestCheckLastObservedIsAfter(t *testing.T) {
	is, _ := testSetup(t)

	observed, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	is.NoErr(err)
	now, err := time.Parse(time.RFC3339, "2006-01-02T15:03:54Z")
	is.NoErr(err)

	is.True(checkLastObservedIsAfter(observed, now, 10))

	now = now.Add(30 * time.Second)
	is.True(!checkLastObservedIsAfter(observed, now, 10))
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
