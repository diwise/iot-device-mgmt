package devicemanagement

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	. "github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

func (d Repository) Seed(ctx context.Context, reader io.Reader, tenants []string) error {
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

	log := logging.GetFromContext(ctx)
	log.Info("loaded devices from file", "count", len(records))

	isAllowed := func(arr []string, s string) bool {
		if len(arr) == 0 {
			return true
		}
		for _, v := range arr {
			if v == s {
				return true
			}
		}
		return false
	}

	for _, record := range records {
		device, _ := record.mapToDevice()

		if !isAllowed(tenants, device.Tenant) {
			log.Warn("tenant not allowed", "device_id", device.DeviceID, "tenant", device.Tenant)
			continue
		}

		e, err := d.GetDeviceByDeviceID(ctx, device.DeviceID, []string{device.Tenant})
		if err != nil {
			err := d.Save(ctx, device)
			if err != nil {
				log.Error("could not create new device", "device_id", device.DeviceID, "err", err.Error())
			}
			continue
		}

		e.Active = device.Active
		e.Description = device.Description
		e.DeviceProfile = device.DeviceProfile
		e.Environment = device.Environment
		e.Location = device.Location
		e.Lwm2mTypes = device.Lwm2mTypes
		e.Name = device.Name
		e.Source = device.Source
		e.Tags = device.Tags
		if e.SensorID != device.SensorID {
			log.Warn("sensorID changed", "device_id", device.DeviceID, "old_sensor_id", e.SensorID, "new_sensor_id", device.SensorID)
			e.SensorID = device.SensorID

		}

		err = d.Save(ctx, e)
		if err != nil {
			log.Error("could not update device", "device_id", device.DeviceID, "err", err.Error())
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
	source      string
}

func (dr deviceRecord) mapToDevice() (Device, DeviceProfile) {
	strArrToLwm2m := func(str []string) []Lwm2mType {
		lw := []Lwm2mType{}
		for _, s := range str {
			lw = append(lw, Lwm2mType{Urn: s})
		}
		return lw
	}

	device := Device{
		Active:      dr.active,
		SensorID:    dr.devEUI,
		DeviceID:    dr.internalID,
		Tenant:      dr.tenant,
		Name:        dr.name,
		Description: dr.description,
		Location: Location{
			Latitude:  dr.lat,
			Longitude: dr.lon,
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
	if !slices.Contains([]string{"", "water", "air", "indoors", "lifebuoy", "soil"}, r.where) {
		return fmt.Errorf("row with %s contains invalid where parameter %s", r.devEUI, r.where)
	}

	if !slices.Contains([]string{"qalcosonic", "sensative", "presence", "elsys", "elsys_codec", "enviot", "senlabt", "tem_lab_14ns", "strips_lora_ms_h", "cube02", "milesight", "milesight_am100", "niab-fls", "virtual", "axsensor", "vegapuls_air_41"}, r.sensorType) {
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
