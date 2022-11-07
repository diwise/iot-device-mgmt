package watchdog

import (
	"context"
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
	app  application.DeviceManagement
	done chan bool
	log  zerolog.Logger
}

func New(app application.DeviceManagement, log zerolog.Logger) Watchdog {
	w := &watchdogImpl{
		app:  app,
		log:  log,
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
	for {
		time.Sleep(10 * time.Second)

		select {
		case <-done:
			return
		default:
			ctx := context.Background()
			devices, err := w.app.ListAllDevices(ctx, []string{"default"})
			if err != nil {
				w.log.Error().Err(err).Msg("could not list all devices")
			}
			for _, d := range devices {
				if d.LastObserved.Before(time.Now().UTC().Add(-time.Duration(d.Intervall) * time.Second)) {
					err := w.app.SetStatusIfChanged(ctx, d.DeviceId, types.DeviceStatus{
						BatteryLevel: 0,
						Code:         types.StatusWarning,
						Messages:     nil,
						Timestamp:    time.Now().UTC().Format(time.RFC3339Nano),
					})
					if err != nil {
						w.log.Error().Err(err).Msgf("could not set status for deviceID %s", d.DeviceId)
					}
				}
			}
		}
	}
}
