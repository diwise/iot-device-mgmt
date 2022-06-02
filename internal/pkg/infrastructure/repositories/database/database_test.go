package database

import (
	"bytes"
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestThatLoadFailsOnDuplicateDeviceEUI(t *testing.T) {
	is := is.New(t)
	_, err := SetUpNewDatabase(zerolog.Logger{}, bytes.NewBuffer([]byte(csvWithDuplicates)))
	is.True(err != nil)
}

func TestThatLoadFailsOnBadLatitude(t *testing.T) {
	is := is.New(t)
	_, err := SetUpNewDatabase(zerolog.Logger{}, bytes.NewBuffer([]byte(csvWithBadLatitude)))
	is.True(err != nil)
}

func TestThatLoadFailsOnBadLongitude(t *testing.T) {
	is := is.New(t)
	_, err := SetUpNewDatabase(zerolog.Logger{}, bytes.NewBuffer([]byte(csvWithBadLongitude)))
	is.True(err != nil)
}

func TestThatLoadFailsOnBadEnvironment(t *testing.T) {
	is := is.New(t)
	_, err := SetUpNewDatabase(zerolog.Logger{}, bytes.NewBuffer([]byte(csvWithBadEnvironment)))
	is.True(err != nil)
}

func TestThatUpdateLastObservedFailsOnOlderTimestamp(t *testing.T) {
	is := is.New(t)
	db, err := SetUpNewDatabase(zerolog.Logger{}, bytes.NewBuffer([]byte(csvWithoutProblems)))
	is.True(err == nil)

	_, err = db.UpdateLastObservedOnDevice("intern-a81758fffe051d00", time.Now().UTC())
	is.True(err == nil) // timestamp should be accepted since no previous lastObserved exists

	_, err = db.UpdateLastObservedOnDevice("intern-a81758fffe051d00", time.Now().UTC().Add(-24*time.Hour))
	is.True(err != nil) // timestamp should not be accepted since it is older than current lastObserved
}

const csvWithoutProblems string = `devEUI;internalID;lat;lon;where;types;sensorType
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec`

const csvWithDuplicates string = `devEUI;internalID;lat;lon;where;types;sensorType
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec
a81758fffe051d00;intern-a81758fffe04d83f;0.0;0.0;ground;urn:oma:lwm2m:ext:3303;Elsys_Codec`

const csvWithBadLatitude string = `devEUI;internalID;lat;lon;where;types;sensorType
a81758fffe051d00;intern-a81758fffe051d00;gurka;0.0;air;urn:oma:lwm2m:ext:3303;Elsys_Codec`

const csvWithBadLongitude string = `devEUI;internalID;lat;lon;where;types;sensorType
a81758fffe051d00;intern-a81758fffe051d00;0.0;gurka;air;urn:oma:lwm2m:ext:3303;Elsys_Codec`

const csvWithBadEnvironment string = `devEUI;internalID;lat;lon;where;types;sensorType
a81758fffe051d00;intern-a81758fffe051d00;0.0;0.0;gurka;urn:oma:lwm2m:ext:3303;Elsys_Codec`
