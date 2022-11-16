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
			tennants, err := w.app.GetTenants(ctx)
			if err != nil {
				w.log.Error().Err(err).Msg("could not fetch tennats")
			}
			devices, err := w.app.GetDevices(ctx, tennants)
			if err != nil {
				w.log.Error().Err(err).Msg("could not list all devices")
			}

			sleepForSeconds := DefaultTimespan

			for _, d := range devices {
				if d.LastObserved.Before(time.Now().UTC().Add(-time.Duration(d.SensorType.Interval) * time.Second)) {
					err = w.app.SetStatus(ctx, d.DeviceID, types.DeviceStatus{
						BatteryLevel: 0,
						Code:         types.StatusWarning,
						Messages:     nil,
						Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
					})
					if err != nil {
						w.log.Error().Err(err).Msgf("could not set status for deviceID %s", d.DeviceID)
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

const ZeroDateTime = -62135596800 // 0000-00-00T00:00:00Z
const DefaultTimespan = 3600

func timeToNextTime(d types.Device, now time.Time) int {
	var t time.Time

	if d.LastObserved.Unix() == ZeroDateTime {
		t = time.Now().UTC()
	} else {
		t = d.LastObserved
	}

	next := t.Add(time.Duration(d.SensorType.Interval) * time.Second)
	n := int(math.Floor(next.Sub(now).Seconds()))

	if n < 0 {
		n = DefaultTimespan
	}

	return n
}
