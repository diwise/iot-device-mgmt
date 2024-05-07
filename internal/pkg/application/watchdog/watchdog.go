package watchdog

import (
	"context"
	"sync"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/devicemanagement"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
)

const DefaultTimespan = 3600

type Watchdog interface {
	Start(context.Context)
	Stop(context.Context)
}

type watchdogImpl struct {
	done             chan bool
	deviceRepository devicemanagement.DeviceRepository
	messenger        messaging.MsgContext
}

func New(d devicemanagement.DeviceRepository, m messaging.MsgContext) Watchdog {
	w := &watchdogImpl{
		done:             make(chan bool),
		deviceRepository: d,
		messenger:        m,
	}

	return w
}

func (w *watchdogImpl) Start(ctx context.Context) {
	go w.run(ctx)
}

func (w *watchdogImpl) Stop(ctx context.Context) {
	w.done <- true
}

func (w *watchdogImpl) run(ctx context.Context) {
	l := &lastObservedWatcher{
		deviceRepository: w.deviceRepository,
		messenger:        w.messenger,
		running:          false,
		interval:         10 * time.Minute,
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

type lastObservedWatcher struct {
	deviceRepository devicemanagement.DeviceRepository
	messenger        messaging.MsgContext
	running          bool
	interval         time.Duration
	mu               sync.Mutex
}

func (l *lastObservedWatcher) Watch(ctx context.Context) {
	ticker := time.NewTicker(l.interval)
	tenants := l.deviceRepository.GetTenants(ctx)
	pub := make(chan string, 0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tenants = l.deviceRepository.GetTenants(ctx)
			go l.checkLastObserved(ctx, tenants, pub)
		case deviceID := <-pub:
			l.publish(ctx, deviceID, tenants)
		}
	}
}

func (l *lastObservedWatcher) setRunning(b bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.running = b
}

func (l *lastObservedWatcher) checkLastObserved(ctx context.Context, tenants []string, pub chan string) {
	if l.running {
		return
	}

	l.setRunning(true)

	offset := 0
	limit := 10

	do := func() bool {
		collection, err := l.deviceRepository.GetOnlineDevices(ctx, offset, limit, tenants)
		if err != nil {
			return false
		}

		for _, d := range collection.Data {
			if !checkLastObservedIsAfter(ctx, d.DeviceStatus.ObservedAt, time.Now(), d.DeviceProfile.Interval) {
				pub <- d.DeviceID
			}
		}

		return collection.Count != 0
	}

	for do() {
		offset = offset + limit
	}

	l.setRunning(false)
}

func checkLastObservedIsAfter(ctx context.Context, lastObserved time.Time, t time.Time, i int) bool {
	shouldHaveBeenCalledAfter := t.Add(-time.Duration(i) * time.Second)
	after := lastObserved.After(shouldHaveBeenCalledAfter)
	return after
}

func (w *lastObservedWatcher) publish(ctx context.Context, deviceID string, tenants []string) {
	logger := logging.GetFromContext(ctx)

	d, err := w.deviceRepository.GetByDeviceID(ctx, deviceID, tenants)
	if err != nil {
		logger.Error("failed to get device by id", "err", err.Error())
		return
	}

	err = w.messenger.PublishOnTopic(ctx, &DeviceNotObserved{
		DeviceID:   deviceID,
		Tenant:     d.Tenant,
		ObservedAt: time.Now().UTC(),
	})
	if err != nil {
		logger.Error("failed to publish last observed", "err", err.Error())
	}
}
