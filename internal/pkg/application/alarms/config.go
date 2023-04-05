package alarms

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
)

type Configuration struct {
	DeviceAlarmConfigurations   []DeviceAlarmConfig
	FunctionAlarmConfigurations []FunctionAlarmConfig
}
type AlarmConfig struct {
	Name     string
	Type     string
	Min      float64
	Max      float64
	Severity int
}
type DeviceAlarmConfig struct {
	DeviceID string
	AlarmConfig
}
type FunctionAlarmConfig struct {
	FunctionID string
	AlarmConfig
}

func loadFile(configFile string) (io.ReadCloser, error) {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("file with known devices (%s) could not be found", configFile)
	}

	f, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("file with known devices (%s) could not be opened", configFile)
	}

	return f, nil
}

func LoadConfiguration(configFile string) *Configuration {
	f, err := loadFile(configFile)
	if err != nil {
		return nil
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.Comma = ';'

	//deviceID;functionID;alarmName;alarmType;min;max;severity
	//deviceID;;batteryLevelChanged;MIN;20;;1
	//deviceID;;deviceNotObserved;MAX;3600;;2
	//;featureID;levelChanged;BETWEEN;20;100;3

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
		DeviceAlarmConfigurations:   make([]DeviceAlarmConfig, 0),
		FunctionAlarmConfigurations: make([]FunctionAlarmConfig, 0),
	}

	for i, row := range rows {
		if i == 0 {
			continue
		}

		cfg := struct {
			DeviceID   string
			FunctionID string
			Name       string
			Type       string
			Min        float64
			Max        float64
			Severity   int
		}{
			DeviceID:   row[0],
			FunctionID: row[1],
			Name:       row[2],
			Type:       row[3],
			Min:        strTof64(row[4]),
			Max:        strTof64(row[5]),
			Severity:   strToInt(row[6], 0),
		}

		if len(cfg.DeviceID) > 0 && len(cfg.FunctionID) == 0 {
			config.DeviceAlarmConfigurations = append(config.DeviceAlarmConfigurations, DeviceAlarmConfig{
				DeviceID: cfg.DeviceID,
				AlarmConfig: AlarmConfig{
					Name:     cfg.Name,
					Type:     cfg.Type,
					Min:      cfg.Min,
					Max:      cfg.Max,
					Severity: cfg.Severity,
				},
			})
		}

		if len(cfg.FunctionID) > 0 && len(cfg.DeviceID) == 0 {
			config.FunctionAlarmConfigurations = append(config.FunctionAlarmConfigurations, FunctionAlarmConfig{
				FunctionID: cfg.FunctionID,
				AlarmConfig: AlarmConfig{
					Name:     cfg.Name,
					Type:     cfg.Type,
					Min:      cfg.Min,
					Max:      cfg.Max,
					Severity: cfg.Severity,
				},
			})
		}
	}

	return &config
}
