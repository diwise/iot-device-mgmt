package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"log/slog"

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

func (f deviceStatus) Body() []byte {
	b, _ := json.Marshal(f)
	return b
}

func (f deviceStatus) ContentType() string {
	return ""
}

func (f deviceStatus) TopicName() string {
	return ""
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

func (f functionUpdated) Body() []byte {
	b, _ := json.Marshal(f)
	return b
}

func (f functionUpdated) ContentType() string {
	return ""
}

func (f functionUpdated) TopicName() string {
	return ""
}

func BatteryLevelChangedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		logger := l.With(slog.String("handler", "BatteryLevelChangedHandler"))

		message := batteryLevelChanged{}

		err := json.Unmarshal(itm.Body(), &message)
		if err != nil {
			logger.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		logger = logger.With(slog.String("device_id", message.DeviceID))
		ctx = logging.NewContextWithLogger(ctx, logger)

		for _, cfg := range as.GetConfiguration().AlarmConfigurations {
			if cfg.ID == message.DeviceID && cfg.Name == AlarmBatteryLevel {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error("could not add batteryLevel alarm", "err", err.Error())
				}
				return
			} else if cfg.ID == "" && cfg.Name == AlarmBatteryLevel {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error("could not add batteryLevel alarm", "err", err.Error())
				}
				return
			}
		}
	}
}

func DeviceNotObservedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		logger := l.With(slog.String("handler", "DeviceNotObservedHandler"))

		message := deviceNotObserved{}

		err := json.Unmarshal(itm.Body(), &message)
		if err != nil {
			logger.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		logger = logger.With(slog.String("device_id", message.DeviceID))
		ctx = logging.NewContextWithLogger(ctx, logger)

		for _, cfg := range as.GetConfiguration().AlarmConfigurations {
			if cfg.ID == message.DeviceID && cfg.Name == AlarmDeviceNotObserved {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error("could not add deviceNotObserved alarm", "err", err.Error())
				}
				return
			} else if cfg.ID == "" && cfg.Name == AlarmDeviceNotObserved {
				err = addAlarm(ctx, as, message.DeviceID, parseDescription(cfg, message.DeviceID, nil), message.Tenant, message.ObservedAt, cfg)
				if err != nil {
					logger.Error("could not add deviceNotObserved alarm", "err", err.Error())
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
			logger.Error("could not add alarm", "err", err.Error())
			return
		}
	}
}

func DeviceStatusHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		logger := l.With(slog.String("handler", "alarms.DeviceStatusHandler"))

		message := deviceStatus{}

		err := json.Unmarshal(itm.Body(), &message)
		if err != nil {
			logger.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		if message.DeviceID == "" {
			logger.Error("no device information")
			return
		}

		logger = logger.With(slog.String("device_id", message.DeviceID))
		ctx = logging.NewContextWithLogger(ctx, logger)

		if message.Code == 0 {
			active_alarms, err := as.GetAlarmsByRefID(ctx, message.DeviceID)
			if err == nil && len(active_alarms) > 0 {
				logger.Info("closing all active alarms for device")
				for _, alarm := range active_alarms {
					as.CloseAlarm(ctx, int(alarm.ID))
				}
			}
			return
		}

		if message.Tenant == "" {
			logger.Error("no tenant information")
			return
		}

		ts, err := time.Parse(time.RFC3339Nano, message.Timestamp)
		if err != nil {
			logger.Error("no valid timestamp", "err", err.Error())
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
					logger.Error("could not add alarm", "err", err.Error())
				}
				return
			} else if cfg.ID == "" && cfg.Name == alarmType() {
				err = addAlarm(ctx, as, message.DeviceID, description(cfg.Description), message.Tenant, ts, cfg)
				if err != nil {
					logger.Error("could not add alarm", "err", err.Error())
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
			logger.Error("could not add alarm", "err", err.Error())
			return
		}
	}
}

func FunctionUpdatedHandler(messenger messaging.MsgContext, as AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		logger := l.With(slog.String("handler", "FunctionUpdatedHandler"))

		f := functionUpdated{}
		err := json.Unmarshal(itm.Body(), &f)
		if err != nil {
			logger.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		logger = logger.With(slog.String("function_id", f.ID))
		ctx = logging.NewContextWithLogger(ctx, logger)

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
				logger.Error("could not handle function updated", "err", err.Error())
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
