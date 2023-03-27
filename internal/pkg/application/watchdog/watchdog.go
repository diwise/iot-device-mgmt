package watchdog

import (
	"context"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog/events"
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/rs/zerolog"
)

const DefaultTimespan = 3600

type Watchdog interface {
	Start()
	Stop()
}
type watchdogImpl struct {
	done             chan bool
	batteryLevel     chan string
	lastObserved     chan string
	log              zerolog.Logger
	deviceRepository db.DeviceRepository
	messenger        messaging.MsgContext
}

func New(d db.DeviceRepository, m messaging.MsgContext, logger zerolog.Logger) Watchdog {
	w := &watchdogImpl{
		done:             make(chan bool),
		batteryLevel:     make(chan string),
		lastObserved:     make(chan string),
		log:              logger,
		deviceRepository: d,
		messenger:        m,
	}

	return w
}

func (w *watchdogImpl) Start() {
	go w.run()
}

func (w *watchdogImpl) Stop() {
	w.done <- true
}

type batteryLevelWatcher struct {
	r db.DeviceRepository
}

func (b *batteryLevelWatcher) Start(ctx context.Context, found chan string) {
	ticker := time.NewTicker(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: get from config
			devices, err := b.r.GetDevices(ctx, "default")
			if err != nil {
				break
			}
			for _, d := range devices {
				// TODO: get from config min level...
				if d.DeviceStatus.BatteryLevel < 20 {
					found <- d.DeviceID
				}
			}
		}
	}
}

type lastObservedWatcher struct {
	r db.DeviceRepository
}

func (l lastObservedWatcher) Start(ctx context.Context, found chan string) {
	ticker := time.NewTicker(10 * time.Minute)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: get from config
			devices, err := l.r.GetDevices(ctx, "default")
			if err != nil {
				break
			}
			for _, d := range devices {
				// TODO: get from config min level...
				if d.DeviceStatus.LastObserved.Before(time.Now().UTC().Add(-time.Duration(10) * time.Minute)) {
					found <- d.DeviceID
				}
			}
		}
	}
}

func (w *watchdogImpl) run() {
	ctx := logging.NewContextWithLogger(context.Background(), w.log)

	b := &batteryLevelWatcher{
		r: w.deviceRepository,
	}
	go b.Start(ctx, w.batteryLevel)

	l := &lastObservedWatcher{
		r: w.deviceRepository,
	}
	go l.Start(ctx, w.lastObserved)

	for {
		select {
		case <-w.done:
			ctx.Done()
			return
		case deviceID := <-w.batteryLevel:
			w.HandleBatteryLevelMessage(ctx, deviceID)
		case deviceID := <-w.lastObserved:
			w.HandleLastObservedMessage(ctx, deviceID)
		}
	}
}

func (w *watchdogImpl) HandleBatteryLevelMessage(ctx context.Context, deviceID string) {
	w.messenger.PublishOnTopic(ctx, &BatteryLevelWarning{
		DeviceID:   deviceID,
		ObservedAt: time.Now().UTC(),
	})
}

func (w *watchdogImpl) HandleLastObservedMessage(ctx context.Context, deviceID string) {
	err := w.messenger.PublishOnTopic(ctx, &LastObservedWarning{
		DeviceID:   deviceID,
		ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		w.log.Error().Err(err).Msg("could not publish lastObservedWarning")
		return
	}
}
