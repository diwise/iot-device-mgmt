package alarms

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
)

type Configuration struct {
	AlarmConfigurations []AlarmConfig
}
type AlarmConfig struct {
	ID          string
	Name        string
	Type        string
	Min         float64
	Max         float64
	Severity    int
	Description string
}

const (
	AlarmBatteryLevel      string = "batteryLevel"
	AlarmDeviceNotObserved string = "deviceNotObserved"

	AlarmTypeMIN     string = "MIN"
	AlarmTypeMAX     string = "MAX"
	AlarmTypeBETWEEN string = "BETWEEN"
	AlarmTypeTRUE    string = "TRUE"
	AlarmTypeFALSE   string = "FALSE"
	AlarmTypeNOOP    string = "-"
)

func loadFile(configFile string) (io.ReadCloser, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("file with alarm configuration (%s) could not be found", configFile)
	}

	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("file with alarm configuration (%s) could not be opened", configFile)
	}

	return f, nil
}

func LoadConfiguration(configFile string) *Configuration {
	f, err := loadFile(configFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	return parseConfigFile(f)
}

func parseConfigFile(f io.Reader) *Configuration {
	r := csv.NewReader(f)
	r.Comma = ';'

	strTof64 := func(s string) float64 {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0.0
		}
		return f
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

	rows, err := r.ReadAll()
	if err != nil {
		return nil
	}

	config := Configuration{
		AlarmConfigurations: make([]AlarmConfig, 0),
	}

	for i, row := range rows {
		if i == 0 {
			continue
		}

		cfg := struct {
			DeviceID    string
			FunctionID  string
			Name        string
			Type        string
			Min         float64
			Max         float64
			Severity    int
			Description string
		}{
			DeviceID:    row[0],
			FunctionID:  row[1],
			Name:        row[2],
			Type:        strings.ToUpper(row[3]),
			Min:         strTof64(row[4]),
			Max:         strTof64(row[5]),
			Severity:    strToInt(row[6], 0),
			Description: row[7],
		}

		if cfg.Type != AlarmTypeNOOP && cfg.Type != AlarmTypeMIN && cfg.Type != AlarmTypeMAX && cfg.Type != AlarmTypeBETWEEN && cfg.Type != AlarmTypeTRUE && cfg.Type != AlarmTypeFALSE {
			continue
		}

		if cfg.Severity != -1 && cfg.Severity != db.AlarmSeverityLow && cfg.Severity != db.AlarmSeverityMedium && cfg.Severity != db.AlarmSeverityHigh {
			continue
		}

		alarmConfig := AlarmConfig{
			Description: cfg.Description,
			Max:         cfg.Max,
			Min:         cfg.Min,
			Name:        cfg.Name,
			Severity:    cfg.Severity,
			Type:        cfg.Type, // MIN, MAX, BETWEEN
		}

		if len(cfg.DeviceID) > 0 && len(cfg.FunctionID) == 0 {
			alarmConfig.ID = cfg.DeviceID
		}

		if len(cfg.DeviceID) == 0 && len(cfg.FunctionID) > 0 {
			alarmConfig.ID = cfg.FunctionID
		}

		config.AlarmConfigurations = append(config.AlarmConfigurations, alarmConfig)
	}

	sort.Slice(config.AlarmConfigurations, func(i, j int) bool {
		return config.AlarmConfigurations[i].ID > config.AlarmConfigurations[j].ID
	})

	return &config
}
