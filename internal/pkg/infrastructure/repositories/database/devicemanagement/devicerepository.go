package devicemanagement

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"gorm.io/gorm"
)

func NewDeviceRepository(connect ConnectorFunc) (DeviceRepository, error) {
	impl, err := connect()
	if err != nil {
		return nil, err
	}

	err = impl.AutoMigrate(&Device{}, &Location{}, &Tenant{}, &DeviceProfile{}, &DeviceStatus{}, &Lwm2mType{}, &DeviceState{}, &Alarm{}, &Tag{})
	if err != nil {
		return nil, err
	}

	return &deviceRepository{
		db: impl,
	}, nil
}

//go:generate moq -rm -out devicerepository_mock.go . DeviceRepository

type DeviceRepository interface {
	GetDevices(ctx context.Context, tenants ...string) ([]Device, error)
	GetOnlineDevices(ctx context.Context, tenants ...string) ([]Device, error)
	GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (Device, error)

	Save(ctx context.Context, device *Device) error

	UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus DeviceStatus) error
	UpdateDeviceState(ctx context.Context, deviceID string, deviceState DeviceState) error

	AddAlarm(ctx context.Context, deviceID string, alarmID int, severity int, observedAt time.Time) error
	RemoveAlarmByID(ctx context.Context, alarmID int) (string, error)

	Seed(context.Context, io.Reader) error
}

var ErrDeviceNotFound = fmt.Errorf("device not found")
var ErrRepositoryError = fmt.Errorf("could not fetch data from repository")

type deviceRepository struct {
	db *gorm.DB
}

func getDevicesQuery(db *gorm.DB) *gorm.DB {
	query := db.Joins("DeviceProfile").Joins("Tenant").Joins("Location").Joins("DeviceStatus").Joins("DeviceState")
	query = query.Preload("Lwm2mTypes")
	query = query.Preload("Tags")

	return query
}

func (d *deviceRepository) GetDevices(ctx context.Context, tenants ...string) ([]Device, error) {
	var devices []Device

	query := getDevicesQuery(d.db)

	if len(tenants) > 0 {
		query = query.Where("tenant_id IN (?)", d.getTenantIDs(ctx, tenants...))
	}

	result := query.Find(&devices)

	return devices, result.Error
}

func (d *deviceRepository) GetOnlineDevices(ctx context.Context, tenants ...string) ([]Device, error) {
	var devices []Device

	query := getDevicesQuery(d.db)
	query = query.Where("online = ?", true)

	if len(tenants) > 0 {
		query = query.Where("tenant_id IN (?)", d.getTenantIDs(ctx, tenants...))
	}

	result := query.Find(&devices)

	return devices, result.Error
}

func (d *deviceRepository) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (Device, error) {
	logger := logging.GetFromContext(ctx)

	var device = Device{}

	query := getDevicesQuery(d.db)
	query = query.Preload("Alarms")
	query = query.Where(&Device{SensorID: strings.ToLower(sensorID)})

	if len(tenants) > 0 {
		query = query.Where("tenant_id IN ?", d.getTenantIDs(ctx, tenants...))
	}

	result := query.First(&device)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return Device{}, ErrDeviceNotFound
		}

		logger.Error("gorm error", "err", result.Error.Error())

		return Device{}, ErrRepositoryError
	}

	return device, nil
}

func (d *deviceRepository) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (Device, error) {
	logger := logging.GetFromContext(ctx)

	var device = Device{}

	query := getDevicesQuery(d.db)
	query = query.Preload("Alarms")
	query = query.Where(&Device{DeviceID: strings.ToLower(deviceID)})

	if len(tenants) > 0 {
		query = query.Where("tenant_id IN ?", d.getTenantIDs(ctx, tenants...))
	}

	result := query.First(&device)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return Device{}, ErrDeviceNotFound
		}

		logger.Error("gorm error", "err", result.Error.Error())

		return Device{}, ErrRepositoryError
	}

	return device, nil
}

func (d *deviceRepository) Save(ctx context.Context, device *Device) error {
	res := d.db.Save(device)
	err := res.Error

	return err
}

func (d *deviceRepository) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus DeviceStatus) error {
	var device Device
	res := d.db.Where(&Device{DeviceID: deviceID}).First(&device)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no such device")
	}

	var ds DeviceStatus
	res = d.db.Where(&DeviceStatus{DeviceID: device.ID}).First(&ds)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		ds = deviceStatus
	} else {
		ds.BatteryLevel = deviceStatus.BatteryLevel
		ds.LastObserved = deviceStatus.LastObserved
	}

	res = d.db.Save(&ds)
	return res.Error
}

func (d *deviceRepository) UpdateDeviceState(ctx context.Context, deviceID string, deviceState DeviceState) error {
	var device Device
	res := d.db.Where(&Device{DeviceID: deviceID}).First(&device)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no such device")
	}

	var ds DeviceState
	res = d.db.Where(&DeviceState{DeviceID: device.ID}).First(&ds)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		ds = deviceState
	} else {
		ds.ObservedAt = deviceState.ObservedAt
		ds.Online = deviceState.Online
		ds.State = deviceState.State
	}

	res = d.db.Save(&ds)
	return res.Error
}

func (d *deviceRepository) AddAlarm(ctx context.Context, deviceID string, alarmID int, severity int, observedAt time.Time) error {
	var device Device
	res := d.db.Where(&Device{DeviceID: deviceID}).First(&device)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("no such device")
	}

	return d.db.Model(&device).Association("Alarms").Append([]Alarm{{AlarmID: alarmID, Severity: severity, ObservedAt: observedAt}})
}

func (d *deviceRepository) RemoveAlarmByID(ctx context.Context, alarmID int) (string, error) {
	var alarm Alarm
	res := d.db.Where(&Alarm{AlarmID: alarmID}).First(&alarm)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", errors.New("no such device")
	}

	var device Device
	res = d.db.Where("id = ?", alarm.DeviceID).First(&device)
	if res.Error != nil {
		return "", res.Error
	}
	if res.RowsAffected == 0 {
		return "", errors.New("no such device")
	}

	err := d.db.Delete(&alarm).Error

	return device.DeviceID, err
}

func (d *deviceRepository) getTenantIDs(ctx context.Context, tenants ...string) []int {
	var ten = []Tenant{}
	d.db.Select("id").Where("name IN ?", tenants).Find(&ten)

	var i []int
	for _, t := range ten {
		i = append(i, int(t.ID))
	}

	return i
}
