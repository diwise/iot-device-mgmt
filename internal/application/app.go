package application

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/application/devices"
	"github.com/diwise/iot-device-mgmt/internal/application/sensors"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

type Management interface {
	DeviceService() devices.DeviceAPIService
	SensorService() sensors.SensorAPIService
	AlarmService() alarms.AlarmAPIService

	SeedLwm2mTypes(ctx context.Context, lwm2m []types.Lwm2mType) error
	SeedSensorProfiles(ctx context.Context, profiles []types.SensorProfile) error
	SeedSensorsAndDevices(ctx context.Context, input io.ReadCloser, validTenants []string, shouldUpdate bool) error
}

type app struct {
	devices devices.DeviceAPIService
	sensors sensors.SensorAPIService
	alarms  alarms.AlarmAPIService
}

func New(devices devices.DeviceAPIService, sensors sensors.SensorAPIService, alarms alarms.AlarmAPIService) Management {
	return &app{
		devices: devices,
		sensors: sensors,
		alarms:  alarms,
	}
}

func (a *app) DeviceService() devices.DeviceAPIService {
	return a.devices
}

func (a *app) SensorService() sensors.SensorAPIService {
	return a.sensors
}

func (a *app) AlarmService() alarms.AlarmAPIService {
	return a.alarms
}

func (a *app) SeedLwm2mTypes(ctx context.Context, lwm2m []types.Lwm2mType) error {
	return a.devices.SeedLwm2mTypes(ctx, lwm2m)
}

func (a *app) SeedSensorProfiles(ctx context.Context, profiles []types.SensorProfile) error {
	return a.devices.SeedSensorProfiles(ctx, profiles)
}

func (a *app) SeedSensorsAndDevices(ctx context.Context, input io.ReadCloser, validTenants []string, shouldUpdate bool) error {

	log := logging.GetFromContext(ctx)
	defer input.Close()

	r := csv.NewReader(input)
	r.Comma = ';'

	rows, err := r.ReadAll()
	if err != nil {
		return err
	}

	records, err := getRecordsFromRows(rows)
	if err != nil {
		return err
	}

	log.Info("loaded devices from file", slog.Int("rows", len(rows)), slog.Int("records", len(records)), slog.Bool("seed_existing_devices", shouldUpdate))

	for _, record := range records {
		device, _ := record.mapToDevice()

		if !slices.Contains(validTenants, device.Tenant) {
			log.Warn("tenant not allowed", "device_id", device.DeviceID, "tenant", device.Tenant)
			continue
		}

		existingSensor, _ := a.existingSensor(ctx, device.SensorID)
		existingDevice, _ := a.existingDevice(ctx, device.DeviceID, validTenants)

		if !existingSensor {
			s := sensors.Sensor{
				SensorID:      device.SensorID,
				SensorProfile: &device.SensorProfile,
			}

			if existingDevice {
				log.Debug("device has sensor that does not exist, seeding sensor", slog.String("device_id", device.DeviceID), slog.String("sensor_id", device.SensorID))
				s.DeviceID = &device.DeviceID
			}

			err := a.sensors.Create(ctx, s)
			if err != nil {
				log.Error("could not seed sensor", "sensor_id", device.SensorID, "decoder", device.SensorProfile.Decoder)
				return err
			}

			log.Debug("seeded new sensor", slog.String("sensor_id", device.SensorID), slog.String("decoder", device.SensorProfile.Decoder))
		} else if shouldUpdate {
			log.Debug("sensor already exists, updating sensor profile if needed", slog.String("sensor_id", device.SensorID), slog.String("decoder", device.SensorProfile.Decoder))

			err := a.sensors.Update(ctx, sensors.Sensor{
				SensorID:      device.SensorID,
				SensorProfile: &device.SensorProfile,
			})
			if err != nil {
				log.Error("could not update sensor", "sensor_id", device.SensorID, "decoder", device.SensorProfile.Decoder, "err", err.Error())
				return err
			}
		}

		if !existingDevice {
			log.Debug("seeding new device", slog.String("device_id", device.DeviceID))

			err := a.devices.Create(ctx, device)
			if err != nil {
				log.Error("could not seed device", "device_id", device.DeviceID, "decoder", device.SensorProfile.Decoder)
				return err
			}
		} else if shouldUpdate {
			log.Debug("device already exists, updating device information if needed", slog.String("device_id", device.DeviceID))

			err := a.devices.Update(ctx, device)
			if err != nil {
				log.Error("could not update device information", "device_id", device.DeviceID, "err", err.Error())
				return err
			}
		}
	}

	return nil
}

func (a *app) existingSensor(ctx context.Context, sensorID string) (bool, error) {
	_, err := a.sensors.Sensor(ctx, sensorID)
	if err != nil {
		if errors.Is(err, sensors.ErrSensorNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (a *app) existingDevice(ctx context.Context, deviceID string, tenants []string) (bool, error) {
	_, err := a.devices.Device(ctx, deviceID, tenants)
	if err != nil {
		if errors.Is(err, devices.ErrDeviceNotFound) {
			return false, nil
		}
		return false, err
	}

	return true, nil
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
	metadata    map[string]string
}

func (dr deviceRecord) mapToDevice() (types.Device, types.SensorProfile) {
	strArrToLwm2m := func(str []string) []types.Lwm2mType {
		lw := []types.Lwm2mType{}
		for _, s := range str {
			lw = append(lw, types.Lwm2mType{Urn: s})
		}
		return lw
	}

	device := types.Device{
		Active:      dr.active,
		SensorID:    strings.TrimSpace(dr.devEUI),
		DeviceID:    strings.TrimSpace(dr.internalID),
		Tenant:      strings.TrimSpace(dr.tenant),
		Name:        strings.TrimSpace(dr.name),
		Description: strings.TrimSpace(dr.description),
		Location: types.Location{
			Latitude:  dr.lat,
			Longitude: dr.lon,
		},
		Environment: strings.TrimSpace(dr.where),
		Source:      strings.TrimSpace(dr.source),
		Lwm2mTypes:  strArrToLwm2m(dr.types),
		SensorProfile: types.SensorProfile{
			Name:    strings.TrimSpace(dr.sensorType),
			Decoder: strings.TrimSpace(dr.sensorType),
		},
		SensorStatus: types.SensorStatus{
			BatteryLevel: -1,
		},
		DeviceState: types.DeviceState{
			Online: false,
			State:  types.DeviceStateUnknown,
		},
		Interval: dr.interval,
	}

	for k, v := range dr.metadata {
		device.Metadata = append(device.Metadata, types.Metadata{
			Key:   k,
			Value: v,
		})
	}

	return device, device.SensorProfile
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

	strToMap := func(str string) map[string]string {
		parts := strings.Split(str, ",")
		m := make(map[string]string)
		for _, part := range parts {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
		return m
	}

	dr := deviceRecord{
		devEUI:      strings.TrimSpace(r[0]),
		internalID:  strings.TrimSpace(r[1]),
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

	if len(r) > 13 {
		dr.metadata = strToMap(r[13])
	} else {
		dr.metadata = make(map[string]string)
	}

	err := validateDeviceRecord(dr)
	if err != nil {
		return deviceRecord{}, err
	}

	return dr, nil
}

func validateDeviceRecord(r deviceRecord) error {
	if !slices.Contains([]string{"", "water", "air", "indoors", "lifebuoy", "soil"}, r.where) {
		return fmt.Errorf("row with %s contains invalid where parameter %s", r.devEUI, r.where)
	}

	if !slices.Contains([]string{
		"qalcosonic",
		"qalcosonic/w1h",
		"qalcosonic/w1t",
		"qalcosonic/w1e",
		"sensative",
		"presence",
		"elsys",
		"elsys/elt/sht3x",
		"elsys_codec",
		"talkpool/oy1210",
		"enviot",
		"senlabt",
		"tem_lab_14ns",
		"strips_lora_ms_h",
		"cube02",
		"milesight",
		"milesight_am100",
		"niab-fls",
		"virtual",
		"axsensor",
		"vegapuls_air_41",
		"airquality"}, r.sensorType) {
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
