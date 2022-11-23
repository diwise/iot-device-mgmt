package watchdog

import (
	"context"
	"math"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/rs/zerolog"
)

type Watchdog interface {
	Start()
	Stop()
}
type watchdogImpl struct {
	done chan bool
	log  zerolog.Logger
	app  application.App
}

func New(app application.App, log zerolog.Logger) Watchdog {
	w := &watchdogImpl{
		log:  log,
		app:  app,
		done: make(chan bool),
	}

	return w
}

func (w *watchdogImpl) Start() {
	go backgroundWorker(w, w.done)
}

func (w *watchdogImpl) Stop() {
	w.done <- true
}

func backgroundWorker(w *watchdogImpl, done <-chan bool) {
	time.Sleep(10 * time.Second)

	for {
		select {
		case <-done:
			return
		default:
			ctx := context.Background()
			tenants, err := w.app.GetTenants(ctx)
			if err != nil || len(tenants) == 0 {
				if err != nil {
					w.log.Error().Err(err).Msg("failed to fetch tenats")
				}
				if len(tenants) == 0 {
					w.log.Error().Err(err).Msg("found 0 tenats")
				}
				w.done <- true
				return
			}
			
			devices, err := w.app.GetDevices(ctx, tenants)
			if err != nil {
				w.log.Error().Err(err).Msg("could not fetch devices!")
				w.done <- true
				return
			}

			sleepForSeconds := DefaultTimespan

			for _, d := range devices {
				interval := d.SensorType.Interval
				if d.Interval > 0 {
					interval = d.Interval
				}

				if d.LastObserved.Before(time.Now().UTC().Add(-time.Duration(interval) * time.Second)) {
					err = w.app.SetStatus(ctx, d.DeviceID, types.DeviceStatus{
						BatteryLevel: -1,
						Code:         types.StatusWarning,
						Messages:     nil,
						Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
					})
					if err != nil {
						w.log.Error().Err(err).Msgf("watchdog could not set status for %s", d.DeviceID)
					}
				}

				nextTime := timeToNextTime(d, time.Now().UTC())
				if nextTime < sleepForSeconds {
					sleepForSeconds = nextTime
				}
			}

			w.log.Debug().Msgf("will sleep for %d seconds", sleepForSeconds)
			time.Sleep(time.Duration(sleepForSeconds) * time.Second)
		}
	}
}

const DefaultTimespan = 3600

func timeToNextTime(d types.Device, now time.Time) int {
	var t time.Time

	if d.LastObserved.IsZero() {
		t = time.Now().UTC()
	} else {
		t = d.LastObserved
	}

	interval := d.SensorType.Interval
	if d.Interval > 0 {
		interval = d.Interval
	}

	next := t.Add(time.Duration(interval) * time.Second)
	n := int(math.Floor(next.Sub(now).Seconds()))

	if n < 0 {
		n = DefaultTimespan
	}

	return n
}
