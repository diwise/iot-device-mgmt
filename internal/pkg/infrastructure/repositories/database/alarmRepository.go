package database

import (
	"context"

	"gorm.io/gorm"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/models"
)

//go:generate moq -rm -out alarmrepository_mock.go . AlarmRepository

type AlarmRepository interface {
	GetAlarms(ctx context.Context, onlyActive bool) ([]Alarm, error)
	AddAlarm(ctx context.Context, alarm Alarm) error
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

func (d *alarmRepository) AddAlarm(ctx context.Context, alarm Alarm) error {
	err := d.db.Debug().WithContext(ctx).
		Save(&alarm).
		Error
	return err
}

func (d *alarmRepository) GetAlarms(ctx context.Context, onlyActive bool) ([]Alarm, error) {
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
