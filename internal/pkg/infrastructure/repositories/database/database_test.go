package database

import (
	"bytes"
	"io"
	"testing"

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

	devs, err := db.GetAll([]string{"default"})
	is.NoErr(err)
	is.Equal(len(devs), 1)

	devs, err = db.GetAll([]string{"default", "notdefault"})
	is.NoErr(err)
	is.Equal(len(devs), 2)
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
