package alarms

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms/events"
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	"github.com/diwise/messaging-golang/pkg/messaging"
)

//go:generate moq -rm -out alarmservice_mock.go . AlarmService
type AlarmService interface {
	Start()
	Stop()

	GetAlarms(ctx context.Context, onlyActive bool) ([]db.Alarm, error)
	AddAlarm(ctx context.Context, alarm db.Alarm) error
	CloseAlarm(ctx context.Context, alarmID int) error

	GetConfiguration() Configuration
}

type alarmService struct {
	alarmRepository db.AlarmRepository
	messenger       messaging.MsgContext
	config          *Configuration
}

func New(d db.AlarmRepository, m messaging.MsgContext, cfg *Configuration) AlarmService {
	as := &alarmService{
		alarmRepository: d,
		messenger:       m,
		config:          cfg,
	}

	as.messenger.RegisterTopicMessageHandler("watchdog.batteryLevelChanged", BatteryLevelChangedHandler(m, as))
	as.messenger.RegisterTopicMessageHandler("watchdog.deviceNotObserved", DeviceNotObservedHandler(m, as))

	return as
}

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

func (a *alarmService) Start() {}
func (a *alarmService) Stop()  {}

func (a *alarmService) GetAlarms(ctx context.Context, onlyActive bool) ([]db.Alarm, error) {
	alarms, err := a.alarmRepository.GetAll(ctx, onlyActive)
	if err != nil {
		return nil, err
	}

	return alarms, nil
}

func (a *alarmService) AddAlarm(ctx context.Context, alarm db.Alarm) error {
	err := a.alarmRepository.Add(ctx, alarm)
	if err != nil {
		return err
	}
	return a.messenger.PublishOnTopic(ctx, &AlarmCreated{Alarm: alarm, Timestamp: time.Now().UTC()})
}

func (a *alarmService) CloseAlarm(ctx context.Context, alarmID int) error {
	err := a.alarmRepository.Close(ctx, alarmID)
	if err != nil {
		return err
	}
	return a.messenger.PublishOnTopic(ctx, &AlarmClosed{ID: alarmID, Timestamp: time.Now().UTC()})
}

func (a *alarmService) GetConfiguration() Configuration {
	return *a.config
}

func BatteryLevelChangedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID     string    `json:"deviceID"`
			BatteryLevel int       `json:"batteryLevel"`
			Tenant       string    `json:"tenant"`
			ObservedAt   time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("deviceID", message.DeviceID).Logger()

		for _, cfg := range as.GetConfiguration().DeviceAlarmConfigurations {
			if strings.EqualFold(cfg.DeviceID, message.DeviceID) {
				if cfg.Name == "batteryLevel" && message.BatteryLevel < int(cfg.Min) {
					err := as.AddAlarm(ctx, db.Alarm{
						RefID: db.AlarmIdentifier{
							DeviceID: message.DeviceID,
						},
						Type:        cfg.Type,
						Severity:    cfg.Severity,
						Active:      true,
						ObservedAt:  time.Now().UTC(),
						Description: fmt.Sprintf("Batterinivå låg %d (min: %d)", message.BatteryLevel, int(cfg.Min)),
					})
					if err != nil {
						logger.Error().Err(err).Msg("could not add alarm")
						return
					}
					break
				}
			}
		}

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}

func DeviceNotObservedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		message := struct {
			DeviceID   string    `json:"deviceID"`
			Tenant     string    `json:"tenant"`
			ObservedAt time.Time `json:"observedAt"`
		}{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("deviceID", message.DeviceID).Logger()

		err = as.AddAlarm(ctx, db.Alarm{
			RefID: db.AlarmIdentifier{
				DeviceID: message.DeviceID,
			},
			Type:        "deviceNotObserved",
			Severity:    1,
			Active:      true,
			ObservedAt:  time.Now().UTC(),
			Description: fmt.Sprintf("Ingen kommunikation registrerad från %s", message.DeviceID),
		})

		if err != nil {
			logger.Error().Err(err).Msg("could not add alarm")
			return
		}

		logger.Debug().Msgf("%s handled", msg.RoutingKey)
	}
}
