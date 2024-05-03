package alarms

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	types "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories"
	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	models "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:generate moq -rm -out alarmrepository_mock.go . AlarmRepository

var ErrAlarmNotFound = fmt.Errorf("alarm not found")

type AlarmRepository interface {
	GetAll(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Alarm], error)
	GetByID(ctx context.Context, alarmID string, tenants []string) (models.Alarm, error)
	GetByRefID(ctx context.Context, refID string, offset, limit int, tenants []string) (types.Collection[models.Alarm], error)
	Add(ctx context.Context, alarm models.Alarm, tenant string) error
	Close(ctx context.Context, alarmID, tenant string) error
}

type Repository struct {
	storage jsonstore.JsonStorage
}

const TypeName string = "Alarm"

const storageConfiguration string = `
serviceName: device-management-alarms
entities:
  - idPattern: ^
    type: Alarm
    tableName: alarms
`

func NewRepository(ctx context.Context, p *pgxpool.Pool) (Repository, error) {
	r := bytes.NewBuffer([]byte(storageConfiguration))
	store, err := jsonstore.NewWithPool(ctx, p, r)
	if err != nil {
		return Repository{}, err
	}

	err = store.Initialize(ctx)
	if err != nil {
		return Repository{}, err
	}

	return Repository{
		storage: store,
	}, nil
}

func (d Repository) GetAll(ctx context.Context, offset, limit int, tenants []string) (types.Collection[models.Alarm], error) {
	result, err := d.storage.FetchType(ctx, TypeName, tenants, jsonstore.Offset(offset), jsonstore.Limit(limit))
	if err != nil {
		return types.Collection[models.Alarm]{}, err
	}
	if result.Count == 0 {
		return types.Collection[models.Alarm]{}, nil
	}

	alarms, err := jsonstore.MapAll[models.Alarm](result.Data)
	if err != nil {
		return types.Collection[models.Alarm]{}, err
	}

	return types.Collection[models.Alarm]{
		Data:       alarms,
		Count:      result.Count,
		Offset:     result.Offset,
		Limit:      result.Limit,
		TotalCount: result.TotalCount,
	}, nil
}

func (d Repository) GetByID(ctx context.Context, alarmID string, tenants []string) (models.Alarm, error) {
	b, err := d.storage.FindByID(ctx, alarmID, TypeName, tenants)
	if err != nil {
		return models.Alarm{}, err
	}

	var alarm models.Alarm
	err = json.Unmarshal(b, &alarm)

	return alarm, err
}

func (d Repository) GetByRefID(ctx context.Context, refID string, offset, limit int, tenants []string) (types.Collection[models.Alarm], error) {
	q := fmt.Sprintf("data ->> 'refID' = '%s'", refID)

	result, err := d.storage.QueryType(ctx, TypeName, q, tenants, jsonstore.Offset(offset), jsonstore.Limit(limit))
	if err != nil {
		return types.Collection[models.Alarm]{}, err
	}
	if result.Count == 0 {
		return types.Collection[models.Alarm]{}, nil
	}

	alarms, err := jsonstore.MapAll[models.Alarm](result.Data)
	if err != nil {
		return types.Collection[models.Alarm]{}, err
	}

	return types.Collection[models.Alarm]{
		Data:       alarms,
		Count:      result.Count,
		Offset:     result.Offset,
		Limit:      result.Limit,
		TotalCount: result.TotalCount,
	}, nil
}

func (d Repository) Add(ctx context.Context, alarm models.Alarm, tenant string) error {
	if alarm.ID == "" {
		alarm.ID = uuid.NewString()
	}

	return jsonstore.Store(ctx, d.storage, alarm, tenant)
}

func (d Repository) Close(ctx context.Context, alarmID, tenant string) error {
	err := d.storage.Delete(ctx, alarmID, TypeName, []string{tenant})
	return err
}
