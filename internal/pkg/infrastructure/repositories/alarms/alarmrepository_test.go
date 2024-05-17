package alarms

import (
	"context"
	"testing"
	"time"

	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	"github.com/diwise/iot-device-mgmt/pkg/types"

	"github.com/google/uuid"
	"github.com/matryer/is"
)

func TestAddAlarm(t *testing.T) {
	is, ctx, repo := testSetup(t)
	err := repo.Add(ctx, types.Alarm{
		ID:          uuid.NewString(),
		Type:        "Alarm",
		AlarmType:   "LastObservedAlarm",
		Description: "Too long since device communicated",
		ObservedAt:  time.Now(),
		RefID:       uuid.NewString(),
		Severity:    1,
		Tenant:      "default",
	}, "default")
	is.NoErr(err)
}

func TestGetByID(t *testing.T) {
	is, ctx, repo := testSetup(t)
	id := uuid.NewString()
	err := repo.Add(ctx, types.Alarm{
		ID:          id,
		Type:        "Alarm",
		AlarmType:   "LastObservedAlarm",
		Description: "Too long since device communicated",
		ObservedAt:  time.Now(),
		RefID:       uuid.NewString(),
		Severity:    1,
		Tenant:      "default",
	}, "default")
	is.NoErr(err)

	a, err := repo.GetByID(ctx, id, []string{"default"})
	is.NoErr(err)
	is.Equal(1, a.Severity)
}

func TestGetByRefID(t *testing.T) {
	is, ctx, repo := testSetup(t)
	id := uuid.NewString()
	refID := uuid.NewString()
	err := repo.Add(ctx, types.Alarm{
		ID:          id,
		Type:        "Alarm",
		AlarmType:   "LastObservedAlarm",
		Description: "Too long since device communicated",
		ObservedAt:  time.Now(),
		RefID:       refID,
		Severity:    1,
		Tenant:      "default",
	}, "default")
	is.NoErr(err)

	collection, err := repo.GetByRefID(ctx, refID, 0, 10, []string{"default"})
	is.NoErr(err)
	is.Equal(id, collection.Data[0].ID)
}

func testSetup(t *testing.T) (*is.I, context.Context, AlarmRepository) {
	is := is.New(t)
	ctx := context.Background()
	config := jsonstore.NewConfig(
		"localhost",
		"postgres",
		"password",
		"5432",
		"postgres",
		"disable",
	)

	p, err := jsonstore.NewPool(ctx, config)
	if err != nil {
		t.SkipNow()
	}

	repo, err := NewRepository(ctx, p)
	if err != nil {
		t.SkipNow()
	}
	
	return is, ctx, repo
}
