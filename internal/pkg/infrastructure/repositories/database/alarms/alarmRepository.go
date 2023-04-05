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
	GetAll(ctx context.Context, onlyActive bool) ([]Alarm, error)
	GetByID(ctx context.Context, alarmID int) (Alarm, error)
	Add(ctx context.Context, alarm Alarm) error
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

	result := d.db.WithContext(ctx).
		Where(&Alarm{ID: uint(alarmID)}).
		First(&a)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return ErrAlarmNotFound
		}
		return result.Error
	}

	a.Active = false
	err := d.db.Debug().WithContext(ctx).
		Save(&a).
		Error

	return err
}

func (d *alarmRepository) Add(ctx context.Context, alarm Alarm) error {
	logger := logging.GetFromContext(ctx)

	a := &Alarm{}

	result := d.db.Debug().WithContext(ctx).
		Where(&Alarm{Type: alarm.Type, RefID: alarm.RefID, Active: true}).
		First(&a)

	if result.RowsAffected > 0 {
		logger.Debug().Msgf("found an active alarm to update time on")
		a.ObservedAt = alarm.ObservedAt
		err := d.db.Debug().WithContext(ctx).
			Save(&a).
			Error
		return err
	}

	logger.Debug().Msg("adding new alarm")

	err := d.db.Debug().WithContext(ctx).
		Save(&alarm).
		Error

	return err
}

func (d *alarmRepository) GetByID(ctx context.Context, alarmID int) (Alarm, error) {
	alarm := &Alarm{}

	err := d.db.WithContext(ctx).
		Where(&Alarm{ID: uint(alarmID)}).
		First(&alarm).
		Error

	if err != nil {
		return Alarm{}, nil
	}

	return *alarm, nil
}

func (d *alarmRepository) GetAll(ctx context.Context, onlyActive bool) ([]Alarm, error) {
	var alarms []Alarm

	query := d.db.WithContext(ctx)

	if onlyActive {
		query = query.Where(&Alarm{Active: true})
	}

	err := query.Find(&alarms).Error
	if err != nil {
		return []Alarm{}, err
	}

	return alarms, nil
}
