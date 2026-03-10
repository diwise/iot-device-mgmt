package devicemanagement

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

	dmquery "github.com/diwise/iot-device-mgmt/internal/application/devicemanagement/query"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func (s service) Seed(ctx context.Context, devices io.ReadCloser, validTenants []string) error {
	return s.importDevices(ctx, devices, validTenants, s.shouldUpdateExistingDevices())
}

func (s service) CreateMany(ctx context.Context, devices io.ReadCloser, validTenants []string) error {
	return s.importDevices(ctx, devices, validTenants, s.shouldUpdateExistingDevices())
}

func (s service) shouldUpdateExistingDevices() bool {
	if s.config == nil {
		return false
	}

	return s.config.SeedExistingDevices
}

func (s service) importDevices(ctx context.Context, devices io.ReadCloser, validTenants []string, shouldUpdate bool) error {

	log := logging.GetFromContext(ctx)
	defer devices.Close()

	r := csv.NewReader(devices)
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

		exists := false
		if strings.TrimSpace(device.SensorID) != "" {
			_, found, err := s.reader.GetDeviceBySensorID(ctx, device.SensorID)
			if err != nil {
				return err
			}
			exists = found
		} else {
			existing, err := s.reader.Query(ctx, dmquery.Devices{Filters: dmquery.Filters{DeviceID: device.DeviceID}})
			if err != nil {
				return err
			}
			exists = existing.Count > 0
		}

		if !exists {
			err := s.writer.CreateOrUpdateDevice(ctx, device)
			if err != nil {
				log.Error("could not seed device", "device_id", device.DeviceID, "decoder", device.SensorProfile.Decoder)
				return err
			}

			log.Debug("seeded new device", slog.String("device_id", device.DeviceID), slog.Bool("seed_existing_devices", shouldUpdate))

			continue
		}

		if !shouldUpdate {
			log.Debug("seed should not update existing devices", slog.String("device_id", device.DeviceID), slog.Bool("seed_existing_devices", shouldUpdate))
			continue
		}

		err = s.writer.CreateOrUpdateDevice(ctx, device)
		if err != nil {
			log.Error("could not seed device", "device_id", device.DeviceID, "decoder", device.SensorProfile.Decoder)
			return err
		}

		log.Debug("updated existing device", slog.String("device_id", device.DeviceID), slog.Bool("seed_existing_devices", shouldUpdate))
	}

	return nil
}

func (s service) SeedLwm2mTypes(ctx context.Context, lwm2m []types.Lwm2mType) error {

	log := logging.GetFromContext(ctx)
	var errs []error
	for _, t := range lwm2m {
		err := s.profiles.CreateSensorProfileType(ctx, t)
		if err != nil {
			log.Debug("failed to seed lwm2m type", "name", t.Name, "urn", t.Urn)
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)

}

func (s service) SeedSensorProfiles(ctx context.Context, profiles []types.SensorProfile) error {

	log := logging.GetFromContext(ctx)
	var errs []error
	for _, p := range profiles {
		err := s.profiles.CreateSensorProfile(ctx, p)
		if err != nil {
			log.Debug("failed to seed device profile", "decoder", p.Decoder, "name", p.Name)
			errs = append(errs, err)
		}
		log.Debug("added device profile", "name", p.Name, "decoder", p.Decoder)
	}
	return errors.Join(errs...)

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
		SensorID:    dr.devEUI,
		DeviceID:    dr.internalID,
		Tenant:      dr.tenant,
		Name:        dr.name,
		Description: dr.description,
		Location: types.Location{
			Latitude:  dr.lat,
			Longitude: dr.lon,
		},
		Environment: dr.where,
		Source:      dr.source,
		Lwm2mTypes:  strArrToLwm2m(dr.types),
		SensorProfile: types.SensorProfile{
			Name:    dr.sensorType,
			Decoder: dr.sensorType,
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
