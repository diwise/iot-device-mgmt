package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
	"github.com/samber/lo"

	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

type batteryLevelChanged struct {
	DeviceID     string    `json:"deviceID"`
	BatteryLevel int       `json:"batteryLevel"`
	Tenant       string    `json:"tenant"`
	ObservedAt   time.Time `json:"observedAt"`
}

type deviceNotObserved struct {
	DeviceID   string    `json:"deviceID"`
	Tenant     string    `json:"tenant"`
	ObservedAt time.Time `json:"observedAt"`
}

type deviceStatus struct {
	DeviceID     string   `json:"deviceID"`
	BatteryLevel int      `json:"batteryLevel"`
	Code         int      `json:"statusCode"`
	Messages     []string `json:"statusMessages,omitempty"`
	Tenant       string   `json:"tenant"`
	Timestamp    string   `json:"timestamp"`
}

type functionUpdated struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	SubType  string `json:"subtype"`
	Location struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location,omitempty"`
	Tenant string `json:"tenant,omitempty"`

	Counter struct {
		Count int  `json:"count"`
		State bool `json:"state"`
	} `json:"counter,omitempty"`

	Level struct {
		Current float64  `json:"current"`
		Percent *float64 `json:"percent,omitempty"`
		Offset  *float64 `json:"offset,omitempty"`
	} `json:"level,omitempty"`

	Presence struct {
		State bool `json:"state"`
	} `json:"presence,omitempty"`

	Timer struct {
		StartTime time.Time      `json:"startTime"`
		EndTime   *time.Time     `json:"endTime,omitempty"`
		Duration  *time.Duration `json:"duration,omitempty"`
		State     bool           `json:"state"`
	} `json:"timer,omitempty"`

	WaterQuality struct {
		Temperature float64 `json:"temperature"`
	} `json:"waterquality,omitempty"`
}

func BatteryLevelChangedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger = logger.With().Str("handler", "BatteryLevelChangedHandler").Logger()

		message := batteryLevelChanged{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("device_id", message.DeviceID).Logger()
		ctx = logging.NewContextWithLogger(ctx, logger)

		for _, cfg := range as.GetConfiguration().AlarmConfigurations {
			if cfg.ID == message.DeviceID && cfg.Name == AlarmBatteryLevel {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error().Err(err).Msg("could not add batteryLevel alarm")
				}
				return
			} else if cfg.ID == "" && cfg.Name == AlarmBatteryLevel {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error().Err(err).Msg("could not add batteryLevel alarm")
				}
				return
			}
		}
	}
}

func DeviceNotObservedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger = logger.With().Str("handler", "DeviceNotObservedHandler").Logger()

		message := deviceNotObserved{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("device_id", message.DeviceID).Logger()
		ctx = logging.NewContextWithLogger(ctx, logger)

		for _, cfg := range as.GetConfiguration().AlarmConfigurations {
			if cfg.ID == message.DeviceID && cfg.Name == AlarmDeviceNotObserved {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error().Err(err).Msg("could not add deviceNotObserved alarm")
				}
				return
			} else if cfg.ID == "" && cfg.Name == AlarmDeviceNotObserved {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error().Err(err).Msg("could not add deviceNotObserved alarm")
				}
				return
			}
		}

		err = as.AddAlarm(ctx, db.Alarm{
			RefID:       message.DeviceID,
			Type:        AlarmDeviceNotObserved,
			Severity:    db.AlarmSeverityMedium,
			Tenant:      message.Tenant,
			ObservedAt:  time.Now().UTC(),
			Description: fmt.Sprintf("Ingen kommunikation registrerad från %s", message.DeviceID),
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not add alarm")
			return
		}
	}
}

func DeviceStatusHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger = logger.With().Str("handler", "alarms.DeviceStatusHandler").Logger()

		message := deviceStatus{}

		err := json.Unmarshal(msg.Body, &message)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		if message.Code == 0 {
			return
		}

		if message.DeviceID == "" {
			logger.Error().Msg("no device information")
			return
		}

		logger = logger.With().Str("device_id", message.DeviceID).Logger()
		ctx = logging.NewContextWithLogger(ctx, logger)

		if message.Tenant == "" {
			logger.Error().Msg("no tenant information")
			return
		}

		ts, err := time.Parse(time.RFC3339Nano, message.Timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("no valid timestamp")
			return
		}

		alarmType := func() string {
			if len(message.Messages) > 0 {
				return message.Messages[0]
			}
			return fmt.Sprintf("%d", message.Code)
		}

		description := func(d string) string {
			if len(message.Messages) > 0 {
				return d + "\n" + strings.Join(message.Messages, "\n")
			}
			return d
		}

		for _, cfg := range as.GetConfiguration().AlarmConfigurations {
			if cfg.ID == message.DeviceID && cfg.Name == alarmType() {
				err = addAlarm(ctx, as, message.DeviceID, description(cfg.Description), message.Tenant, ts, cfg)
				if err != nil {
					logger.Error().Err(err).Msg("could not add alarm")
				}
				return
			} else if cfg.ID == "" && cfg.Name == alarmType() {
				err = addAlarm(ctx, as, message.DeviceID, description(cfg.Description), message.Tenant, ts, cfg)
				if err != nil {
					logger.Error().Err(err).Msg("could not add alarm")
				}
				return
			}
		}

		_, _, err = lo.AttemptWithDelay(3, 1*time.Second, func(index int, duration time.Duration) error {
			return as.AddAlarm(ctx, db.Alarm{
				RefID:       message.DeviceID,
				Type:        alarmType(),
				Severity:    db.AlarmSeverityLow,
				Tenant:      message.Tenant,
				ObservedAt:  ts,
				Description: description(""),
			})
		})
		if err != nil {
			logger.Error().Err(err).Msg("could not add alarm")
			return
		}
	}
}

func FunctionUpdatedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger = logger.With().Str("handler", "FunctionUpdatedHandler").Logger()

		f := functionUpdated{}
		err := json.Unmarshal(msg.Body, &f)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal message")
			return
		}

		logger = logger.With().Str("function_id", f.ID).Logger()

		for _, cfg := range as.GetConfiguration().AlarmConfigurations {
			if cfg.ID != "" && cfg.ID != f.ID {
				continue
			} else if cfg.ID == "" && cfg.Name != f.Type {
				continue
			}

			switch cfg.Type {
			case AlarmTypeMIN:
				err = alarmTypeMinHandler(ctx, f, cfg, as)
			case AlarmTypeMAX:
				err = alarmTypeMaxHandler(ctx, f, cfg, as)
			case AlarmTypeBETWEEN:
				err = alarmTypeBetweenHandler(ctx, f, cfg, as)
			case AlarmTypeTRUE:
				err = alarmTypeTrueHandler(ctx, f, cfg, as)
			case AlarmTypeFALSE:
				err = alarmTypeFalseHandler(ctx, f, cfg, as)
			}

			if err != nil {
				logger.Error().Err(err).Msg("could not handle function updated")
			}
		}
	}
}

func parseDescription(cfg AlarmConfig, id string, val any) string {
	desc := cfg.Description
	desc = strings.ReplaceAll(desc, "{MIN}", fmt.Sprintf("%g", cfg.Min))
	desc = strings.ReplaceAll(desc, "{MAX}", fmt.Sprintf("%g", cfg.Max))
	if id != "" {
		desc = strings.ReplaceAll(desc, "{ID}", id)
	}
	if val != nil {
		desc = strings.ReplaceAll(desc, "{VALUE}", fmt.Sprintf("%v", val))
	}
	return desc
}

func addAlarm(ctx context.Context, as AlarmService, id, desc, tenant string, ts time.Time, cfg AlarmConfig) error {
	if cfg.Severity == -1 {
		return nil
	}

	err := as.AddAlarm(ctx, db.Alarm{
		RefID:       id,
		Type:        cfg.Name,
		Severity:    cfg.Severity,
		Tenant:      tenant,
		ObservedAt:  ts,
		Description: desc,
	})

	return err
}

const (
	FuncTypeCounter      string = "counter"
	FuncTypeLevel        string = "level"
	FuncTypeWaterQuality string = "waterquality"
	FuncTypeTimer        string = "timer"
	FuncTypePresence     string = "presence"
)

func alarmTypeMinHandler(ctx context.Context, f functionUpdated, cfg AlarmConfig, as AlarmService) error {
	switch f.Type {
	case FuncTypeCounter:
		if f.Counter.Count < int(cfg.Min) {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Counter.Count), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeLevel:
		if *f.Level.Percent < cfg.Min {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, *f.Level.Percent), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeWaterQuality:
		if f.WaterQuality.Temperature < cfg.Min {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.WaterQuality.Temperature), f.Tenant, time.Now().UTC(), cfg)
		}
	default:
		return fmt.Errorf("not implemented")
	}
	return nil
}

func alarmTypeMaxHandler(ctx context.Context, f functionUpdated, cfg AlarmConfig, as AlarmService) error {
	switch f.Type {
	case FuncTypeCounter:
		if f.Counter.Count > int(cfg.Max) {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Counter.Count), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeLevel:
		if *f.Level.Percent > cfg.Max {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, *f.Level.Percent), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeWaterQuality:
		if f.WaterQuality.Temperature > cfg.Max {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.WaterQuality.Temperature), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeTimer:
		if f.Timer.State && f.Timer.Duration != nil && *f.Timer.Duration > time.Duration(cfg.Max) {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, *f.Timer.Duration), f.Tenant, time.Now().UTC(), cfg)
		}
	default:
		return fmt.Errorf("not implemented")
	}
	return nil
}

func alarmTypeBetweenHandler(ctx context.Context, f functionUpdated, cfg AlarmConfig, as AlarmService) error {
	switch f.Type {
	case FuncTypeCounter:
		if !(f.Counter.Count > int(cfg.Min) && f.Counter.Count < int(cfg.Max)) {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Counter.Count), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeLevel:
		if !(*f.Level.Percent > cfg.Min && *f.Level.Percent < cfg.Max) {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, *f.Level.Percent), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeWaterQuality:
		if !(f.WaterQuality.Temperature > cfg.Min && f.WaterQuality.Temperature < cfg.Max) {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.WaterQuality.Temperature), f.Tenant, time.Now().UTC(), cfg)
		}
	default:
		return fmt.Errorf("not implemented")
	}
	return nil
}

func alarmTypeTrueHandler(ctx context.Context, f functionUpdated, cfg AlarmConfig, as AlarmService) error {
	switch f.Type {
	case FuncTypeCounter:
		if f.Counter.State {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Counter.State), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypePresence:
		if f.Presence.State {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Presence.State), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypeTimer:
		if f.Timer.State {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Timer.State), f.Tenant, time.Now().UTC(), cfg)
		}
	default:
		return fmt.Errorf("not implemented")
	}
	return nil
}

func alarmTypeFalseHandler(ctx context.Context, f functionUpdated, cfg AlarmConfig, as AlarmService) error {
	switch f.Type {
	case FuncTypeCounter:
		if !f.Counter.State {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Counter.State), f.Tenant, time.Now().UTC(), cfg)
		}
	case FuncTypePresence:
		if !f.Presence.State {
			return addAlarm(ctx, as, f.ID, parseDescription(cfg, f.ID, f.Presence.State), f.Tenant, time.Now().UTC(), cfg)
		}
	default:
		return fmt.Errorf("not implemented")
	}
	return nil
}
