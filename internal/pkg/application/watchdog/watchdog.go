package watchdog

import (
	"context"
	"time"

	. "github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog/events"
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
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
	log              zerolog.Logger
	deviceRepository db.DeviceRepository
	messenger        messaging.MsgContext
}

func New(d db.DeviceRepository, m messaging.MsgContext, logger zerolog.Logger) Watchdog {
	w := &watchdogImpl{
		done:             make(chan bool),
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

func (w *watchdogImpl) run() {
	ctx := logging.NewContextWithLogger(context.Background(), w.log)

	b := &batteryLevelWatcher{
		deviceRepository: w.deviceRepository,
		batteryLevels:    map[string]int{},
		messenger:        w.messenger,
	}
	go b.Watch(ctx)

	l := &lastObservedWatcher{
		deviceRepository: w.deviceRepository,
		messenger:        w.messenger,
	}
	go l.Watch(ctx)

	for range w.done {
		ctx.Done()
		return
	}
}

type Watcher interface {
	Watch(ctx context.Context)
}

type batteryLevelWatcher struct {
	deviceRepository db.DeviceRepository
	batteryLevels    map[string]int
	messenger        messaging.MsgContext
}

func (b *batteryLevelWatcher) Watch(ctx context.Context) {
	b.batteryLevels = make(map[string]int)

	ticker := time.NewTicker(30 * time.Minute)
	logger := logging.GetFromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			changed, err := b.checkBatteryLevels(ctx)
			if err != nil {
				logger.Error().Err(err).Msg("could not check batteryLevels")
			}

			for _, c := range changed {
				err := b.publish(ctx, c)
				if err != nil {
					logger.Error().Err(err).Msg("could not publish BatteryLevelChanged")
				}
			}
		}
	}
}

func (b *batteryLevelWatcher) checkBatteryLevels(ctx context.Context) ([]string, error) {
	devices, err := b.deviceRepository.GetOnlineDevices(ctx)
	if err != nil {
		return nil, err
	}

	changedDevices := []string{}

	for _, d := range devices {
		if level, ok := b.batteryLevels[d.DeviceID]; ok {
			if d.DeviceStatus.BatteryLevel > 0 && d.DeviceStatus.BatteryLevel != level {
				changedDevices = append(changedDevices, d.DeviceID)
			}
		} else {
			b.batteryLevels[d.DeviceID] = d.DeviceStatus.BatteryLevel
		}
	}

	return changedDevices, nil
}

func (b *batteryLevelWatcher) publish(ctx context.Context, deviceID string) error {
	d, err := b.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	err = b.messenger.PublishOnTopic(ctx, &BatteryLevelChanged{
		DeviceID:     deviceID,
		BatteryLevel: d.DeviceStatus.BatteryLevel,
		Tenant:       d.Tenant.Name,
		ObservedAt:   time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	return nil
}

type lastObservedWatcher struct {
	deviceRepository db.DeviceRepository
	messenger        messaging.MsgContext
}

func (l *lastObservedWatcher) Watch(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	logger := logging.GetFromContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checked, err := l.checkLastObserved(ctx, logger)
			if err != nil {
				logger.Error().Err(err).Msg("failed to check last observed")
				break
			}
			for _, c := range checked {
				err := l.publish(ctx, c)
				if err != nil {
					logger.Error().Err(err).Msg("failed to publish last observed")
					break
				}
			}
		}
	}
}

func (l *lastObservedWatcher) checkLastObserved(ctx context.Context, logger zerolog.Logger) ([]string, error) {
	devices, err := l.deviceRepository.GetOnlineDevices(ctx)
	if err != nil {
		return nil, err
	}

	checkedDeviceIDs := []string{}

	for _, d := range devices {
		if !checkLastObservedIsAfter(logger, d.DeviceStatus.LastObserved.UTC(), time.Now().UTC(), d.DeviceProfile.Interval) {
			logger.Debug().Msgf("lastObserved status on %s with profile %s and interval %d seconds", d.DeviceID, d.DeviceProfile.Name, d.DeviceProfile.Interval)
			checkedDeviceIDs = append(checkedDeviceIDs, d.DeviceID)
		}
	}

	return checkedDeviceIDs, nil
}

func checkLastObservedIsAfter(logger zerolog.Logger, lastObserved time.Time, t time.Time, i int) bool {
	shouldHaveBeenCalledAfter := t.Add(-time.Duration(i) * time.Second)
	after := lastObserved.After(shouldHaveBeenCalledAfter)
	logger.Debug().Msgf("lastObserved: %s, after:%s, return: %t", lastObserved.Format(time.RFC3339Nano), shouldHaveBeenCalledAfter.Format(time.RFC3339Nano), after)
	return after
}

func (w *lastObservedWatcher) publish(ctx context.Context, deviceID string) error {
	d, err := w.deviceRepository.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}

	err = w.messenger.PublishOnTopic(ctx, &DeviceNotObserved{
		DeviceID:   deviceID,
		Tenant:     d.Tenant.Name,
		ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	return nil
}
