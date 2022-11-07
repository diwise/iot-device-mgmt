package database

import (
	"bytes"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

const devices string = `devEUI;internalID;lat;lon;where;types;sensorType;name;description;active;tenant
a81758fffe06bfa3;intern-a81758fffe06bfa3;62.39160;17.30723;water;urn:oma:lwm2m:ext:3303,urn:oma:lwm2m:ext:3302,urn:oma:lwm2m:ext:3301;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;default
a81758fffe05e6fb;intern-a81758fffe05e6fb;62.39160;17.30723;;urn:oma:lwm2m:ext:3428;Elsys_Codec;name-a81758fffe06bfa3;desc-a81758fffe06bfa3;true;notdefault
`

func TestThatGetAllRetrievesByTenantNames(t *testing.T) {
	testData := bytes.NewBuffer([]byte(devices))
	is, db := testSetup(t, testData)

	devs, err := db.GetAll("default")
	is.NoErr(err)
	is.Equal(len(devs), 1)

	devs, err = db.GetAll("default", "notdefault")
	is.NoErr(err)
	is.Equal(len(devs), 2)
}

func TestConcurrentAccess(t *testing.T) {
	testData := bytes.NewBuffer([]byte(devices))
	is, db := testSetup(t, testData)

	const numThreads int = 100

	var wg sync.WaitGroup

	wg.Add(numThreads)

	accessor := func() {
		defer wg.Done()

		for i := 0; i < 100; i++ {
			err := db.UpdateLastObservedOnDevice("a81758fffe06bfa3", time.Now().UTC())
			if err != nil {
				is.NoErr(err)
				return
			}
		}
	}

	for t := 0; t < numThreads; t++ {
		go accessor()
	}

	wg.Wait()
}

func TestSetStatusIfChanged_NewStatus(t *testing.T) {
	is, db := testSetup(t, bytes.NewBuffer([]byte(devices)))

	sm := Status{
		DeviceID:     "intern-a81758fffe06bfa3",
		BatteryLevel: 100,
		Status:       0,
		Messages:     "",
		Timestamp:    time.Now().Format(time.RFC3339Nano),
	}

	err := db.SetStatusIfChanged(sm)
	is.NoErr(err)
}

func TestSetStatusIfChanged_UpdateStatus(t *testing.T) {
	is, db := testSetup(t, bytes.NewBuffer([]byte(devices)))

	sm := Status{
		DeviceID:     "intern-a81758fffe06bfa3",
		BatteryLevel: 100,
		Status:       0,
		Messages:     "",
		Timestamp:    time.Now().Format(time.RFC3339Nano),
	}

	err := db.SetStatusIfChanged(sm)
	is.NoErr(err)

	sm.BatteryLevel = 98
	sm.Timestamp = time.Now().Format(time.RFC3339Nano)
	err = db.SetStatusIfChanged(sm)
	is.NoErr(err)
}

func TestGetStatus(t *testing.T) {
	is, db := testSetup(t, bytes.NewBuffer([]byte(devices)))

	sm := Status{
		DeviceID:     "intern-a81758fffe06bfa3",
		BatteryLevel: 100,
		Status:       0,
		Messages:     "",
		Timestamp:    time.Now().Format(time.RFC3339Nano),
	}

	err := db.SetStatusIfChanged(sm)
	is.NoErr(err)

	sm2, err := db.GetLatestStatus("intern-a81758fffe06bfa3")
	is.NoErr(err)

	is.Equal(sm.DeviceID, sm2.DeviceID)
	is.Equal(100, sm2.BatteryLevel)

	sm.BatteryLevel = 98
	sm.Timestamp = time.Now().Format(time.RFC3339Nano)
	err = db.SetStatusIfChanged(sm)
	is.NoErr(err)

	sm3, err := db.GetLatestStatus("intern-a81758fffe06bfa3")
	is.NoErr(err)
	is.Equal(98, sm3.BatteryLevel)
}

func testSetup(t *testing.T, testData io.Reader) (*is.I, Datastore) {
	is := is.New(t)
	log := zerolog.Logger{}
	db, err := NewDatabaseConnection(NewSQLiteConnector(log))
	is.NoErr(err)

	db.Seed(testData)
	is.NoErr(err)

	return is, db
}
