package database

import (
	"context"
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/models"
	"github.com/matryer/is"
)

func TestAddAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)

	err := r.AddAlarm(ctx, models.Alarm{
		RefID:       models.AlarmIdentifier{DeviceID: "deviceID"},
		Type:        "type",
		Severity:    models.AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})

	is.NoErr(err)
}

func TestAddTwoAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)

	alarms, _ := r.GetAlarms(ctx, false)
	l := len(alarms)

	err := r.AddAlarm(ctx, models.Alarm{
		RefID:       models.AlarmIdentifier{DeviceID: "deviceID"},
		Type:        "type",
		Severity:    models.AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})

	is.NoErr(err)

	err = r.AddAlarm(ctx, models.Alarm{
		RefID:       models.AlarmIdentifier{DeviceID: "deviceID"},
		Type:        "type",
		Severity:    models.AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})

	is.NoErr(err)

	alarms, _ = r.GetAlarms(ctx, false)

	is.Equal(l+1, len(alarms))
}

func TestGetAlarms(t *testing.T) {
	is, ctx, r := testSetupAlarmRepository(t)
	err := r.AddAlarm(ctx, models.Alarm{
		RefID:       models.AlarmIdentifier{DeviceID: "deviceID"},
		Type:        "type",
		Severity:    models.AlarmSeverityHigh,
		Description: "desc",
		Active:      true,
		ObservedAt:  time.Now(),
	})
	is.NoErr(err)

	alarms, err := r.GetAlarms(ctx, true)
	is.NoErr(err)

	is.True(len(alarms) > 0)

}

func testSetupAlarmRepository(t *testing.T) (*is.I, context.Context, AlarmRepository) {
	is, ctx, conn := setup(t)

	r, _ := NewAlarmRepository(conn)

	return is, ctx, r
}

/*
func TestGetAlarms(t *testing.T) {
	is, ctx, r := testSetup(t)

	err := r.Save(ctx, createDevice(10, "default"))
	is.NoErr(err)

	alarms, err := r.GetAlarms(ctx, true)
	is.NoErr(err)
	is.True(alarms != nil)
}

func TestAddAlarms(t *testing.T) {
	is, ctx, r := testSetup(t)

	err := r.Save(ctx, createDevice(99, "default"))
	is.NoErr(err)

	alarms, err := r.GetAlarms(ctx, true)
	is.NoErr(err)
	l := len(alarms)

	r.AddAlarm(ctx, "device-99", Alarm{
		Type:        "type",
		Severity:    AlarmSeverityHigh,
		Active:      true,
		Description: "description",
		ObservedAt:  time.Now(),
	})

	alarms, err = r.GetAlarms(ctx, true)
	is.NoErr(err)
	is.Equal(l+1, len(alarms))
	is.Equal(AlarmSeverityHigh, alarms[l].Severity)
}
*/
