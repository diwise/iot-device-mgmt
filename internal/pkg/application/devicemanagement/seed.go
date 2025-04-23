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

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)



func SeedDevices(ctx context.Context, s *storage.Storage, devices io.ReadCloser, validTenants []string) error {
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

	log.Info("loaded devices from file", slog.Int("rows", len(rows)), slog.Int("records", len(records)))

	for _, record := range records {
		device, _ := record.mapToDevice()

		if !slices.Contains(validTenants, device.Tenant) {
			log.Warn("tenant not allowed", "device_id", device.DeviceID, "tenant", device.Tenant)
			continue
		}

		err := s.CreateOrUpdateDevice(ctx, device)
		if err != nil {
			return err
		}
	}
	return nil
}

func SeedLwm2mTypes(ctx context.Context, s *storage.Storage, lwm2m []types.Lwm2mType) error {
	var errs []error
	for _, t := range lwm2m {
		errs = append(errs, s.CreateDeviceProfileType(ctx, t))
	}
	return errors.Join(errs...)
}

func SeedDeviceProfiles(ctx context.Context, s *storage.Storage, profiles []types.DeviceProfile) error {
	var errs []error
	for _, p := range profiles {
		errs = append(errs, s.CreateDeviceProfile(ctx, p))
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
}

func (dr deviceRecord) mapToDevice() (types.Device, types.DeviceProfile) {
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
		DeviceProfile: types.DeviceProfile{
			Name:     dr.sensorType,
			Interval: dr.interval,
		},
		DeviceStatus: types.DeviceStatus{
			BatteryLevel: -1,
		},
		DeviceState: types.DeviceState{
			Online: false,
			State:  types.DeviceStateUnknown,
		},
	}

	return device, device.DeviceProfile
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

	if !slices.Contains([]string{"qalcosonic", "sensative", "presence", "elsys", "elsys_codec", "enviot", "senlabt", "tem_lab_14ns", "strips_lora_ms_h", "cube02", "milesight", "milesight_am100", "niab-fls", "virtual", "axsensor", "vegapuls_air_41", "airquality"}, r.sensorType) {
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
