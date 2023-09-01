package alarms

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

//go:generate moq -rm -out alarmrepository_mock.go . AlarmRepository

var ErrAlarmNotFound = fmt.Errorf("alarm not found")

type AlarmRepository interface {
	GetAll(ctx context.Context, tenants ...string) ([]Alarm, error)
	GetByID(ctx context.Context, alarmID int) (Alarm, error)
	GetByRefID(ctx context.Context, refID string) ([]Alarm, error)
	Add(ctx context.Context, alarm Alarm) (int, error)
	Close(ctx context.Context, alarmID int) error
}

type alarmRepository struct {
	db *gorm.DB
}

func NewAlarmRepository(connect ConnectorFunc) (AlarmRepository, error) {
	impl, _, err := connect()
	if err != nil {
		return nil, err
	}

	err = impl.AutoMigrate(&Alarm{})
	if err != nil {
		return nil, err
	}

	return &alarmRepository{
		db: impl,
	}, nil
}

func (d *alarmRepository) Close(ctx context.Context, alarmID int) error {
	a := Alarm{}

	result := d.db.
		Where(&Alarm{ID: uint(alarmID)}).
		First(&a)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrAlarmNotFound
		}
		return result.Error
	}

	err := d.db.Delete(&a).Error

	return err
}

func (d *alarmRepository) Add(ctx context.Context, alarm Alarm) (int, error) {
	logger := logging.GetFromContext(ctx)

	a := &Alarm{}

	result := d.db.Where(&Alarm{Type: alarm.Type, RefID: alarm.RefID}).First(&a)

	if result.RowsAffected > 0 {
		err := d.db.Model(&a).Update("observed_at", alarm.ObservedAt).Error
		return int(a.ID), err
	}

	logger.Debug().Msgf("add new alarm, refID: %s, type: %s, tenant: %s", alarm.RefID, alarm.Type, alarm.Tenant)

	result = d.db.Create(&alarm)
	if result.Error != nil {
		return 0, result.Error
	}

	return int(alarm.ID), nil
}

func (d *alarmRepository) GetByID(ctx context.Context, alarmID int) (Alarm, error) {
	alarm := Alarm{}

	err := d.db.First(&alarm, alarmID).Error

	if err != nil {
		return Alarm{}, err
	}

	return alarm, nil
}

func (d *alarmRepository) GetByRefID(ctx context.Context, refID string) ([]Alarm, error) {
	alarms := []Alarm{}

	err := d.db.Where(&Alarm{RefID: refID}).Find(&alarms).Error

	if err != nil {
		return []Alarm{}, err
	}

	return alarms, nil
}

func (d *alarmRepository) GetAll(ctx context.Context, tenants ...string) ([]Alarm, error) {
	alarms := []Alarm{}

	err := d.db.Find(&alarms).Error
	if err != nil {
		return []Alarm{}, err
	}

	return alarms, nil
}
