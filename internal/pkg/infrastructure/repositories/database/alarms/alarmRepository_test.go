package alarms

import (
	"context"
	"testing"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/google/uuid"
	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestAddAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)

	err := r.Add(ctx, Alarm{
		RefID:       "deviceID",
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})

	is.NoErr(err)
}

func TestAddTwoAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)

	alarms, _ := r.GetAll(ctx, false)
	l := len(alarms)

	deviceID := uuid.New().String()

	err := r.Add(ctx, Alarm{
		RefID:       "deviceID",
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})

	is.NoErr(err)

	err = r.Add(ctx, Alarm{
		RefID:       deviceID,
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})

	is.NoErr(err)

	alarms, _ = r.GetAll(ctx, false)

	is.Equal(l+1, len(alarms))
}

func TestGetAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)
	err := r.Add(ctx, Alarm{
		RefID:       "deviceID",
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)

	alarms, err := r.GetAll(ctx, true)
	is.NoErr(err)

	is.True(len(alarms) > 0)

}

func testSetupAlarmRepository(t *testing.T) (*is.I, context.Context, AlarmRepository) {
	is, ctx, conn := setup(t)

	r, _ := NewAlarmRepository(conn)

	return is, ctx, r
}

func setup(t *testing.T) (*is.I, context.Context, ConnectorFunc) {
	is := is.New(t)

	conn := NewSQLiteConnector(zerolog.Logger{})

	return is, context.Background(), conn
}
