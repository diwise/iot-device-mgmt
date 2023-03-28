package database

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/models"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/rs/zerolog"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type ConnectorFunc func() (*gorm.DB, zerolog.Logger, error)

func NewSQLiteConnector(log zerolog.Logger) ConnectorFunc {
	return func() (*gorm.DB, zerolog.Logger, error) {
		db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
			Logger:          logger.Default.LogMode(logger.Silent),
			CreateBatchSize: 1000,
		})

		if err == nil {
			db.Exec("PRAGMA foreign_keys = ON")
			sqldb, _ := db.DB()
			sqldb.SetMaxOpenConns(1)
		}

		return db, log, err
	}
}

func NewPostgreSQLConnector(log zerolog.Logger) ConnectorFunc {
	dbHost := os.Getenv("DIWISE_SQLDB_HOST")
	username := os.Getenv("DIWISE_SQLDB_USER")
	dbName := os.Getenv("DIWISE_SQLDB_NAME")
	password := os.Getenv("DIWISE_SQLDB_PASSWORD")
	sslMode := env.GetVariableOrDefault(log, "DIWISE_SQLDB_SSLMODE", "require")

	dbURI := fmt.Sprintf("host=%s user=%s dbname=%s sslmode=%s password=%s", dbHost, username, dbName, sslMode, password)

	return func() (*gorm.DB, zerolog.Logger, error) {
		sublogger := log.With().Str("host", dbHost).Str("database", dbName).Logger()

		for {
			sublogger.Info().Msg("connecting to database host")

			db, err := gorm.Open(postgres.Open(dbURI), &gorm.Config{
				Logger: logger.New(
					&sublogger,
					logger.Config{
						SlowThreshold:             time.Second,
						LogLevel:                  logger.Info,
						IgnoreRecordNotFoundError: false,
						Colorful:                  false,
					},
				),
			})
			if err != nil {
				sublogger.Fatal().Err(err).Msg("failed to connect to database")
				time.Sleep(3 * time.Second)
			} else {
				return db, sublogger, nil
			}
		}
	}
}

func New(connect ConnectorFunc) (DeviceRepository, error) {
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
	GetDeviceID(ctx context.Context, sensorID string) (string, error)
	GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string, tenants ...string) (Device, error)
	GetStatistics(ctx context.Context) (DeviceStatistics, error)

	Save(ctx context.Context, device *Device) error

	UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus DeviceStatus) error
	UpdateDeviceState(ctx context.Context, deviceID string, deviceState DeviceState) error

	GetAlarms(ctx context.Context, onlyActive bool) ([]Alarm, error)
	AddAlarm(ctx context.Context, deviceID string, alarm Alarm) error

	Seed(context.Context, string, io.Reader) error
}

var ErrDeviceNotFound = fmt.Errorf("device not found")
var ErrRepositoryError = fmt.Errorf("could not fetch data from repository")

type deviceRepository struct {
	db *gorm.DB
}

func (d *deviceRepository) GetDevices(ctx context.Context, tenants ...string) ([]Device, error) {
	var devices []Device

	result := d.db.WithContext(ctx).
		Preload("Tenant").
		Preload("Location").
		Preload("Lwm2mTypes").
		Preload("DeviceProfile").
		Preload("DeviceStatus").
		Preload("DeviceState").
		Preload("Tags").
		Where("tenant_id IN (?)", d.getTenantIDs(ctx, tenants...)).
		Find(&devices)

	return devices, result.Error
}

func (d *deviceRepository) getTenantIDs(ctx context.Context, tenants ...string) []int {
	var ten = []Tenant{}
	d.db.WithContext(ctx).
		Select("id").
		Where("name IN ?", tenants).
		Find(&ten)

	var i []int
	for _, t := range ten {
		i = append(i, int(t.ID))
	}

	return i
}

func (d *deviceRepository) GetDeviceID(ctx context.Context, sensorID string) (string, error) {
	var device = Device{}

	result := d.db.WithContext(ctx).
		Where(&Device{SensorID: sensorID}).
		First(&device)

	return device.DeviceID, result.Error
}

func (d *deviceRepository) GetDeviceBySensorID(ctx context.Context, sensorID string, tenants ...string) (Device, error) {
	logger := logging.GetFromContext(ctx)

	var device = Device{}

	query := d.db.WithContext(ctx)

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

	query := d.db.WithContext(ctx).
		Preload("Location").
		Preload("Tenant").
		Preload("DeviceProfile").
		Preload("Lwm2mTypes").
		Preload("DeviceStatus").
		Preload("DeviceState").
		Preload("Tags").
		Preload("Alarms")

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

	err := d.db.WithContext(ctx).
		Debug().
		Session(&gorm.Session{FullSaveAssociations: true}).
		Save(device).
		Error

	return err
}

func (d *deviceRepository) UpdateDeviceStatus(ctx context.Context, deviceID string, deviceStatus DeviceStatus) error {
	var device = Device{}
	err := d.db.WithContext(ctx).
		Preload("DeviceStatus").
		Where(&Device{DeviceID: deviceID}).
		First(&device).
		Error

	if err != nil {
		return err
	}

	device.DeviceStatus = deviceStatus

	err = d.db.Save(&device).Error
	if err != nil {
		return err
	}

	return nil
}

func (d *deviceRepository) UpdateDeviceState(ctx context.Context, deviceID string, deviceState DeviceState) error {
	var device = Device{}
	err := d.db.WithContext(ctx).
		Preload("DeviceState").
		Where(&Device{DeviceID: deviceID}).
		First(&device).
		Error

	if err != nil {
		return err
	}

	device.DeviceState = deviceState

	err = d.db.Save(&device).Error
	if err != nil {
		return err
	}

	return nil
}

func (d *deviceRepository) GetAlarms(ctx context.Context, onlyActive bool) ([]Alarm, error) {
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

func (d *deviceRepository) AddAlarm(ctx context.Context, deviceID string, alarm Alarm) error {
	device, err := d.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	device.Alarms = append(device.Alarms, alarm)

	return d.Save(ctx, &device)
}

func (d *deviceRepository) GetStatistics(ctx context.Context) (DeviceStatistics, error) {
	return DeviceStatistics{}, nil
}

func (d *deviceRepository) Seed(ctx context.Context, key string, reader io.Reader) error {
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
		Lwm2mTypes:  strArrToLwm2m(dr.types),
		DeviceProfile: DeviceProfile{
			Name:    dr.sensorType,
			Decoder: dr.sensorType,
		},
		DeviceStatus: DeviceStatus{
			BatteryLevel: -1,			
		},
		DeviceState: DeviceState{
			Online: false,
			State: DeviceStateUnknown,
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
		if n, err := strconv.Atoi(r[11]); err == nil {
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
