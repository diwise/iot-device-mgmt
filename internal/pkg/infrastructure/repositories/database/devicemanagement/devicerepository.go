package devicemanagement

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"gorm.io/gorm"
)

func NewDeviceRepository(connect ConnectorFunc) (DeviceRepository, error) {
	impl, _, err := connect()
	if err != nil {
		return nil, err
	}

	err = impl.AutoMigrate(&Device{}, &Location{}, &Tenant{}, &DeviceProfile{}, &DeviceStatus{}, &Lwm2mType{}, &DeviceState{}, &Alarm{})
	if err != nil {
		return nil, err
	}

	return &deviceRepository{
		_db: impl,
	}, nil
}

//go:generate moq -rm -out devicerepository_mock.go . DeviceRepository

type DeviceRepository interface {
	GetDevices(ctx context.Context, tenants ...string) ([]Device, error)
	GetOnlineDevices(ctx context.Context, tenants ...string) ([]Device, error)
	GetDeviceID(ctx context.Context, sensorID string) (string, error)
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
	_db *gorm.DB
}

func (d *deviceRepository) Db(ctx context.Context) (tx *gorm.DB) {
	return d._db.WithContext(ctx)
}

func (d *deviceRepository) getDeviceQuery(ctx context.Context) (tx *gorm.DB) {
	return d.Db(ctx).
		Preload("Location").
		Preload("Tenant").
		Preload("DeviceProfile").
		Preload("Lwm2mTypes").
		Preload("DeviceStatus").
		Preload("DeviceState").
		Preload("Tags").
		Preload("Alarms")
}

func (d *deviceRepository) GetDevices(ctx context.Context, tenants ...string) ([]Device, error) {
	var devices []Device

	query := d.getDeviceQuery(ctx)

	if len(tenants) > 0 {
		query = query.Where("tenant_id IN (?)", d.getTenantIDs(ctx, tenants...))
	}

	result := query.Find(&devices)

	return devices, result.Error
}

func (d *deviceRepository) GetOnlineDevices(ctx context.Context, tenants ...string) ([]Device, error) {
	var devices []Device

	query := d.getDeviceQuery(ctx)

	if len(tenants) > 0 {
		query = query.Where("tenant_id IN (?)", d.getTenantIDs(ctx, tenants...))
	}

	result := query.Find(&devices)

	online := []Device{}
	for _, device := range devices {
		if device.DeviceState.Online {
			online = append(online, device)
		}
	}

	return online, result.Error
}

func (d *deviceRepository) GetDeviceID(ctx context.Context, sensorID string) (string, error) {
	var device = Device{}

	result := d.Db(ctx).
		Where(&Device{SensorID: sensorID}).
		First(&device)

	return device.DeviceID, result.Error
}

func (d *deviceRepository) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (Device, error) {
	logger := logging.GetFromContext(ctx)

	var device = Device{}

	query := d.getDeviceQuery(ctx)

	if len(tenants) == 0 {
		query = query.Where(&Device{SensorID: sensorID})
	} else {
		t := d.getTenantIDs(ctx, tenants...)
		query = query.Where("sensor_id = ? AND tenant_id IN ?", sensorID, t)
	}

	result := query.First(&device)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return Device{}, ErrDeviceNotFound
		}

		logger.Error().Err(result.Error).Msg("gorm error")

		return Device{}, ErrRepositoryError
	}

	return device, nil
}

func (d *deviceRepository) GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (Device, error) {
	logger := logging.GetFromContext(ctx)

	var device = Device{}
	query := d.getDeviceQuery(ctx)

	if len(tenants) == 0 {
		query = query.Where(&Device{DeviceID: deviceID})
	} else {
		t := d.getTenantIDs(ctx, tenants...)
		query = query.Where("device_id = ? AND tenant_id IN ?", deviceID, t)
	}

	result := query.First(&device)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return Device{}, ErrDeviceNotFound
		}

		logger.Error().Err(result.Error).Msg("gorm error")

		return Device{}, ErrRepositoryError
	}

	return device, nil
}

func (d *deviceRepository) Save(ctx context.Context, device *Device) error {
	if device.ID == 0 {
		fromDb, err := d.GetDeviceByDeviceID(ctx, device.DeviceID)
		if err != nil {
			if !errors.Is(err, ErrDeviceNotFound) {
				return nil
			}
		} else {
			device.ID = fromDb.ID
		}
	}

	err := d.Db(ctx).
		Session(&gorm.Session{FullSaveAssociations: true}).
		Save(device).
		Error

	return err
}

func (d *deviceRepository) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus DeviceStatus) error {
	var device = Device{}
	err := d.Db(ctx).
		Preload("DeviceStatus").
		Where(&Device{DeviceID: deviceID}).
		First(&device).
		Error

	if err != nil {
		return err
	}

	device.DeviceStatus = deviceStatus

	err = d.Db(ctx).Save(&device).Error
	if err != nil {
		return err
	}

	return nil
}

func (d *deviceRepository) UpdateDeviceState(ctx context.Context, deviceID string, deviceState DeviceState) error {
	var device = Device{}
	err := d.Db(ctx).
		Preload("DeviceState").
		Where(&Device{DeviceID: deviceID}).
		First(&device).
		Error

	if err != nil {
		return err
	}

	device.DeviceState = deviceState

	err = d.Db(ctx).Save(&device).Error
	if err != nil {
		return err
	}

	return nil
}

func (d *deviceRepository) AddAlarm(ctx context.Context, deviceID string, alarmID int, severity int, observedAt time.Time) error {
	device := Device{}

	result := d.Db(ctx).
		Preload("Alarms").
		Where(&Device{DeviceID: deviceID}).
		First(&device)
	if result.RowsAffected == 0 || errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.Alarms = append(device.Alarms, Alarm{AlarmID: alarmID, Severity: severity, ObservedAt: observedAt})

	return d.Db(ctx).Save(&device).Error
}

func (d *deviceRepository) RemoveAlarmByID(ctx context.Context, alarmID int) (string, error) {
	a := Alarm{}

	result := d.Db(ctx).
		Where(&Alarm{AlarmID: alarmID}).
		First(&a)

	if result.RowsAffected == 0 || errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", nil
	}

	device := Device{}
	err := d.Db(ctx).
		First(&device, a.DeviceID).
		Error
	if err != nil {
		return "", err
	}

	err = d.Db(ctx).
		Delete(&a).
		Error

	return device.DeviceID, err
}

func (d *deviceRepository) Seed(ctx context.Context, reader io.Reader) error {
	r := csv.NewReader(reader)
	r.Comma = ';'

	rows, err := r.ReadAll()
	if err != nil {
		return err
	}

	records, err := getRecordsFromRows(rows)
	if err != nil {
		return err
	}

	for _, record := range records {
		device := record.Device()
		err := d.Save(ctx, &device)
		if err != nil {
			return err
		}
	}

	return nil
}

func (d *deviceRepository) getTenantIDs(ctx context.Context, tenants ...string) []int {
	var ten = []Tenant{}
	d.Db(ctx).
		Select("id").
		Where("name IN ?", tenants).
		Find(&ten)

	var i []int
	for _, t := range ten {
		i = append(i, int(t.ID))
	}

	return i
}

type deviceRecord struct {
	devEUI      string
	internalID  string
	lat         float64
	lon         float64
	where       string
	types       []string
	sensorType  string
	name        string
	description string
	active      bool
	tenant      string
	interval    int
	source      string
}

func (dr deviceRecord) Device() Device {
	strArrToLwm2m := func(str []string) []Lwm2mType {
		lw := []Lwm2mType{}
		for _, s := range str {
			lw = append(lw, Lwm2mType{Urn: s})
		}
		return lw
	}

	return Device{
		Active:   dr.active,
		SensorID: dr.devEUI,
		DeviceID: dr.internalID,
		Tenant: Tenant{
			Name: dr.tenant,
		},
		Name:        dr.name,
		Description: dr.description,
		Location: Location{
			Latitude:  dr.lat,
			Longitude: dr.lon,
			Altitude:  0.0,
		},
		Environment: dr.where,
		Source:      dr.source,
		Lwm2mTypes:  strArrToLwm2m(dr.types),
		DeviceProfile: DeviceProfile{
			Name:     dr.sensorType,
			Decoder:  dr.sensorType,
			Interval: dr.interval,
		},
		DeviceStatus: DeviceStatus{
			BatteryLevel: -1,
		},
		DeviceState: DeviceState{
			Online: false,
			State:  DeviceStateUnknown,
		},
	}
}

func newDeviceRecord(r []string) (deviceRecord, error) {
	strTof64 := func(s string) float64 {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0.0
		}
		return f
	}

	strToArr := func(str string) []string {
		arr := strings.Split(str, ",")
		for i, a := range arr {
			arr[i] = strings.ToLower(a)
		}
		return arr
	}

	strToBool := func(str string) bool {
		return str == "true"
	}

	strToInt := func(str string, def int) int {
		if n, err := strconv.Atoi(str); err == nil {
			if n == 0 {
				return def
			}
			return n
		}
		return def
	}

	dr := deviceRecord{
		devEUI:      strings.ToLower(r[0]),
		internalID:  strings.ToLower(r[1]),
		lat:         strTof64(r[2]),
		lon:         strTof64(r[3]),
		where:       r[4],
		types:       strToArr(r[5]),
		sensorType:  r[6],
		name:        r[7],
		description: r[8],
		active:      strToBool(r[9]),
		tenant:      r[10],
		interval:    strToInt(r[11], 3600),
		source:      r[12],
	}

	err := validateDeviceRecord(dr)
	if err != nil {
		return deviceRecord{}, err
	}

	return dr, nil
}

func validateDeviceRecord(r deviceRecord) error {
	contains := func(s string, arr []string) bool {
		for _, a := range arr {
			if strings.EqualFold(s, a) {
				return true
			}
		}
		return false
	}

	if !contains(r.where, []string{"", "water", "air", "indoors", "lifebuoy", "soil"}) {
		return fmt.Errorf("row with %s contains invalid where parameter %s", r.devEUI, r.where)
	}

	if !contains(r.sensorType, []string{"qalcosonic", "presence", "elsys_codec", "enviot", "tem_lab_14ns", "strips_lora_ms_h", "cube02", "milesight_am100", "niab-fls", "virtual"}) {
		return fmt.Errorf("row with %s contains invalid sensorType parameter %s", r.devEUI, r.sensorType)
	}

	return nil
}

func getRecordsFromRows(rows [][]string) ([]deviceRecord, error) {
	records := []deviceRecord{}

	for i, row := range rows {
		if i == 0 {
			continue
		}
		rec, err := newDeviceRecord(row)
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}

	return records, nil
}
