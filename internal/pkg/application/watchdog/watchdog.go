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
	ticker := time.NewTicker(30 * time.Second)
	logger := logging.GetFromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: get from config
			devices, err := b.r.GetOnlineDevices(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("could not check batteryLevel")
				break
			}

			logger.Debug().Msgf("checking batteryLevel status on %d devices...", len(devices))

			for _, d := range devices {
				logger.Debug().Msgf("checking batteryLevel on %s, battery level: %d", d.DeviceID, d.DeviceStatus.BatteryLevel)

				// TODO: get from config min level...
				if d.DeviceStatus.BatteryLevel > 0 && d.DeviceStatus.BatteryLevel < 20 {
					logger.Debug().Msgf("batteryLevel is %d, publish alarm", d.DeviceStatus.BatteryLevel)
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
	ticker := time.NewTicker(10 * time.Second)
	logger := logging.GetFromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// TODO: get from config
			devices, err := l.r.GetOnlineDevices(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("could not check lastObserved, failed to get devices")
				break
			}

			logger.Debug().Msgf("checking lastObserved status on %d devices...", len(devices))
			for _, d := range devices {
				logger.Debug().Msgf("checking lastObserved status on %s with profile %s and interval %d seconds", d.DeviceID, d.DeviceProfile.Name, d.DeviceProfile.Interval)

				// TODO: get from config min level...
				if !checkLastObservedIsAfter(logger, d.DeviceStatus.LastObserved.UTC(), time.Now().UTC(), d.DeviceProfile.Interval) {
					found <- d.DeviceID
				}
			}
		}
	}
}

func checkLastObservedIsAfter(logger zerolog.Logger, lastObserved time.Time, t time.Time, i int) bool {
	shouldHaveBeenCalledAfter := t.Add(-time.Duration(i) * time.Second)
	after := lastObserved.After(shouldHaveBeenCalledAfter)
	logger.Debug().Msgf("lastObserved: %s, after:%s, return: %t", lastObserved.Format(time.RFC3339Nano), shouldHaveBeenCalledAfter.Format(time.RFC3339Nano), after)
	return after
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
	d, err := w.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		w.log.Error().Err(err).Msg("could not publish batteryLevelWarning, device not found")
		return
	}

	err = w.messenger.PublishOnTopic(ctx, &BatteryLevelWarning{
		DeviceID:   deviceID,
		Tenant:     d.Tenant.Name,
		ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		w.log.Error().Err(err).Msg("could not publish batteryLevelWarning")
		return
	}

	w.log.Debug().Msgf("BatteryLevelWarning published for %s", deviceID)
}

func (w *watchdogImpl) HandleLastObservedMessage(ctx context.Context, deviceID string) {
	d, err := w.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		w.log.Error().Err(err).Msg("could not publish lastObservedWarning, device not found")
		return
	}

	err = w.messenger.PublishOnTopic(ctx, &LastObservedWarning{
		DeviceID:   deviceID,
		Tenant:     d.Tenant.Name,
		ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		w.log.Error().Err(err).Msg("could not publish lastObservedWarning")
		return
	}

	w.log.Debug().Msgf("LastObservedWarning published for %s", deviceID)
}
