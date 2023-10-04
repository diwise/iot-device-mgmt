package devicemanagement

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

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

	log := logging.GetFromContext(ctx)
	log.Info("loaded devices from file", "count", len(records))

	for _, record := range records {
		device := record.Device()

		_, err := d.GetDeviceByDeviceID(ctx, device.DeviceID)
		if errors.Is(err, ErrDeviceNotFound) {
			err := d.Save(ctx, &device)
			if err != nil {
				log.Error("could not seed device", "device_id", device.DeviceID, "err", err.Error())
			}
		} else if err != nil {
			log.Error("unable to check if device exists", "device_id", device.DeviceID, "err", err.Error())
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
