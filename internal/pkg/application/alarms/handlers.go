package alarms

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

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

var tracer = otel.Tracer("iot-device-mgmt/alarm")

func NewDeviceNotObservedHandler(messenger messaging.MsgContext, svc AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "device-not-observed")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		msg := deviceNotObserved{}

		err = json.Unmarshal(itm.Body(), &msg)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		alarms, err := svc.GetByRefID(ctx, msg.DeviceID, 0, 100, []string{msg.Tenant})
		if err != nil {
			log.Error("could not fetch alarms", "err", err.Error())
			return
		}
		if alarms.TotalCount > alarms.Count {
			log.Warn("too many alarms found")
		}

		addNew := true

		for _, a := range alarms.Data {
			if a.AlarmType == AlarmDeviceNotObserved {
				if msg.ObservedAt.After(a.ObservedAt) {
					a.ObservedAt = msg.ObservedAt
					err := svc.Add(ctx, a)
					if err != nil {
						log.Error("could not update alarm", "alarm_id", a.ID, "err", err.Error())
						return
					}
				}
				addNew = false
			}
		}

		if addNew {
			err := svc.Add(ctx, types.Alarm{
				ID:          uuid.NewString(),				
				AlarmType:   AlarmDeviceNotObserved,
				Description: fmt.Sprintf("ingen kommunikation registerad"),
				ObservedAt:  msg.ObservedAt,
				RefID:       msg.DeviceID,
				Severity:    1,
				Tenant:      msg.Tenant,
			})
			if err != nil {
				log.Error("could not create alarm", "err", err.Error())
				return
			}
		}
	}
}

func NewDeviceStatusHandler(messenger messaging.MsgContext, svc AlarmService) messaging.TopicMessageHandler {
	return func(ctx context.Context, itm messaging.IncomingTopicMessage, l *slog.Logger) {
		var err error

		ctx, span := tracer.Start(ctx, "device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log := o11y.AddTraceIDToLoggerAndStoreInContext(span, l, ctx)

		status := deviceStatus{}

		err = json.Unmarshal(itm.Body(), &status)
		if err != nil {
			log.Error("failed to unmarshal message", "err", err.Error())
			return
		}

		alarms, err := svc.GetByRefID(ctx, status.DeviceID, 0, 100, []string{status.Tenant})
		if err != nil {
			log.Error("could not fetch alarms", "err", err.Error())
			return
		}

		if alarms.TotalCount > alarms.Count {
			log.Warn("too many alarms found")
		}

		if status.Code == 0 {
			for _, a := range alarms.Data {
				if a.AlarmType == AlarmDeviceNotObserved {
					err := svc.Close(ctx, a.ID, []string{a.Tenant})
					if err != nil {
						log.Error("could not close alarm", "alarm_id", a.ID, "err", err.Error())
						continue
					}
				}
			}
			return
		}

		for _, m := range status.Messages {
			existing := false
			for _, a := range alarms.Data {
				if a.AlarmType == m {
					existing = true
					if ts, err := time.Parse(time.RFC3339, status.Timestamp); err == nil {
						if ts.After(a.ObservedAt) {
							a.ObservedAt = ts
							err := svc.Add(ctx, a)
							if err != nil {
								log.Error("could not update alarm", "alarm_id", a.ID, "err", err.Error())
								continue
							}
						}
					}
				}
			}
			if !existing {
				ts, err := time.Parse(time.RFC3339, status.Timestamp)
				if err != nil {
					ts = time.Now().UTC()
				}
				err = svc.Add(ctx, types.Alarm{
					ID:          uuid.NewString(),					
					AlarmType:   m,
					Description: "",
					ObservedAt:  ts,
					RefID:       status.DeviceID,
					Severity:    1,
					Tenant:      status.Tenant,
				})
			}
		}
	}
}
