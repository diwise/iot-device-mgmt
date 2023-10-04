package alarms

import (
	"context"
	"testing"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestAddAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)

	i, err := r.Add(ctx, Alarm{
		RefID:       "deviceID",
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		ObservedAt:  time.Now(),
	})

	is.NoErr(err)
	is.True(i > 0)
}

func TestAddTwoAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)

	alarms, _ := r.GetAll(ctx)
	l := len(alarms)

	_, err := r.Add(ctx, Alarm{
		RefID:       uuid.New().String(),
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)

	deviceID := uuid.New().String()

	_, err = r.Add(ctx, Alarm{
		RefID:       deviceID,
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)

	alarms, _ = r.GetAll(ctx)
	is.Equal(l+2, len(alarms))

	_, err = r.Add(ctx, Alarm{
		RefID:       deviceID,
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)

	alarms, _ = r.GetAll(ctx)
	is.Equal(l+2, len(alarms))
}

func TestGetAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)
	i, err := r.Add(ctx, Alarm{
		RefID:       "deviceID",
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)
	is.True(i > 0)

	alarms, err := r.GetAll(ctx)
	is.NoErr(err)

	is.True(len(alarms) > 0)

}

func TestGetByID(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)
	i, err := r.Add(ctx, Alarm{
		RefID:       "deviceID",
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)
	is.True(i > 0)

	alarms, err := r.GetAll(ctx)
	is.NoErr(err)
	is.True(len(alarms) > 0)

	alarmByID, err := r.GetByID(ctx, int(alarms[0].ID))
	is.NoErr(err)
	is.True(alarmByID.ID > 0)
}

func TestGetByRefID(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)
	i, err := r.Add(ctx, Alarm{
		RefID:       "deviceID",
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Description: "desc",
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)
	is.True(i > 0)

	alarms, err := r.GetByRefID(ctx, "deviceID")
	is.NoErr(err)
	is.True(len(alarms) > 0 && alarms[0].ID > 0 && alarms[0].Severity == AlarmSeverityHigh)
}

func testSetupAlarmRepository(t *testing.T) (*is.I, context.Context, AlarmRepository) {
	is, ctx, conn := setup(t)

	r, _ := NewAlarmRepository(conn)

	return is, ctx, r
}

func setup(t *testing.T) (*is.I, context.Context, ConnectorFunc) {
	is := is.New(t)
	ctx := context.Background()
	conn := NewSQLiteConnector(ctx)

	return is, ctx, conn
}
