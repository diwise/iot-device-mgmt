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
		query = query.Where("devices.tenant_id IN (?)", d.getTenantIDs(ctx, tenants...))
	}

	result := query.Find(&devices)

	return devices, result.Error
}

func (d *deviceRepository) GetOnlineDevices(ctx context.Context, tenants ...string) ([]Device, error) {
	var devices []Device

	query := getDevicesQuery(d.db)

	if len(tenants) > 0 {
		query = query.Where("devices.tenant_id IN (?)", d.getTenantIDs(ctx, tenants...))
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

func (d *deviceRepository) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (Device, error) {
	logger := logging.GetFromContext(ctx)

	var device = Device{}

	query := getDevicesQuery(d.db)
	query = query.Preload("Alarms")

	if len(tenants) == 0 {
		query = query.Where(&Device{SensorID: strings.ToLower(sensorID)})
	} else {
		t := d.getTenantIDs(ctx, tenants...)
		query = query.Where("devices.sensor_id = ? AND devices.tenant_id IN ?", strings.ToLower(sensorID), t)
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
	query := getDevicesQuery(d.db)
	query = query.Preload("Alarms")

	if len(tenants) == 0 {
		query = query.Where(&Device{DeviceID: strings.ToLower(deviceID)})
	} else {
		t := d.getTenantIDs(ctx, tenants...)
		query = query.Where("devices.device_id = ? AND devices.tenant_id IN ?", strings.ToLower(deviceID), t)
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

	tx := d.db.Session(&gorm.Session{
		FullSaveAssociations:   true,
		SkipDefaultTransaction: true,
	})

	tx = tx.Save(device)
	err := tx.Error

	return err
}

func (d *deviceRepository) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus DeviceStatus) error {
	var device = Device{}

	// find device.id from device_id
	r := d.db.Select("id").Where("device_id = ?", deviceID).First(&device)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return errors.New("no such device")
	}

	var storedStatus = DeviceStatus{}

	r = d.db.Where(&DeviceStatus{DeviceID: device.ID}).First(&storedStatus)
	if r.Error != nil {
		if errors.Is(r.Error, gorm.ErrRecordNotFound) {
			// no such device state exists, so lets create one
			deviceStatus.DeviceID = device.ID
			return d.db.Save(&deviceStatus).Error
		}
		return r.Error
	}

	// update the stored state with the new values
	storedStatus.BatteryLevel = deviceStatus.BatteryLevel
	storedStatus.LastObserved = deviceStatus.LastObserved

	return d.db.Save(&storedStatus).Error
}

func (d *deviceRepository) UpdateDeviceState(ctx context.Context, deviceID string, deviceState DeviceState) error {
	var device = Device{}

	// find device.id from device_id
	r := d.db.Select("id").Where("device_id = ?", deviceID).First(&device)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return errors.New("no such device")
	}

	var storedState = DeviceState{}

	// find device_state from device_id
	r = d.db.Where(&DeviceState{DeviceID: device.ID}).First(&storedState)
	if r.Error != nil {
		if errors.Is(r.Error, gorm.ErrRecordNotFound) {
			// no such device state exists, so lets create one
			deviceState.DeviceID = device.ID
			return d.db.Save(&deviceState).Error
		}
		return r.Error
	}

	// update the stored state with the new values
	storedState.Online = deviceState.Online
	storedState.State = deviceState.State
	storedState.ObservedAt = deviceState.ObservedAt

	return d.db.Save(&storedState).Error
}

func (d *deviceRepository) AddAlarm(ctx context.Context, deviceID string, alarmID int, severity int, observedAt time.Time) error {
	device := Device{}

	result := d.db.
		Preload("Alarms").
		Where(&Device{DeviceID: strings.ToLower(deviceID)}).
		First(&device)
	if result.RowsAffected == 0 || errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("device %s not found", deviceID)
	}

	device.Alarms = append(device.Alarms, Alarm{AlarmID: alarmID, Severity: severity, ObservedAt: observedAt})

	return d.db.Save(&device).Error
}

func (d *deviceRepository) RemoveAlarmByID(ctx context.Context, alarmID int) (string, error) {
	a := Alarm{}

	result := d.db.
		Where(&Alarm{AlarmID: alarmID}).
		First(&a)

	if result.RowsAffected == 0 || errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", nil
	}

	device := Device{}
	err := d.db.
		First(&device, a.DeviceID).
		Error
	if err != nil {
		return "", err
	}

	err = d.db.
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
	d.db.
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
		sensorType:  strings.ToLower(r[6]),
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

	if !contains(r.sensorType, []string{"qalcosonic", "sensative", "presence", "elsys", "elsys_codec", "enviot", "senlabt", "tem_lab_14ns", "strips_lora_ms_h", "cube02", "milesight", "milesight_am100", "niab-fls", "virtual"}) {
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
