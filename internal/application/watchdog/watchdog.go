package watchdog

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	"github.com/diwise/iot-device-mgmt/pkg/types"
)

const DefaultTimespan = 3600

type WatchdogConfig struct {
	Interval int `yaml:"interval"`
}

type Watchdog interface {
	Start(context.Context)
	Stop(context.Context)
}

type watchdogImpl struct {
	mu       sync.Mutex
	running  atomic.Bool
	cancel   context.CancelFunc
	done     chan struct{}
	watchers []Watcher
}

func New(a alarms.AlarmService, cfg *WatchdogConfig) Watchdog {
	interval := 10 * time.Minute
	if cfg != nil && cfg.Interval > 0 {
		interval = time.Duration(cfg.Interval) * time.Minute
	}

	w := &watchdogImpl{
		watchers: []Watcher{
			&lastObservedWatcher{
				alarmSvc: a,
				interval: interval,
			},
		},
	}

	return w
}

func (w *watchdogImpl) Start(ctx context.Context) {
	if !w.running.CompareAndSwap(false, true) {
		return
	}

	watchCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	w.mu.Lock()
	w.cancel = cancel
	w.done = done
	watchers := append([]Watcher(nil), w.watchers...)
	w.mu.Unlock()

	go func() {
		defer w.running.Store(false)
		defer close(done)

		var wg sync.WaitGroup
		for _, watcher := range watchers {
			wg.Add(1)
			go func(watcher Watcher) {
				defer wg.Done()
				watcher.Watch(watchCtx)
			}(watcher)
		}

		wg.Wait()
	}()
}

func (w *watchdogImpl) Stop(ctx context.Context) {
	w.mu.Lock()
	cancel := w.cancel
	done := w.done
	w.mu.Unlock()

	if cancel == nil {
		return
	}

	cancel()

	if done == nil {
		return
	}

	select {
	case <-done:
	case <-ctx.Done():
	}

	w.mu.Lock()
	if w.done == done {
		w.done = nil
		w.cancel = nil
	}
	w.mu.Unlock()
}

type Watcher interface {
	Watch(ctx context.Context)
}

type lastObservedWatcher struct {
	alarmSvc alarms.AlarmService
	running  atomic.Bool
	interval time.Duration
}

func (l *lastObservedWatcher) Watch(ctx context.Context) {
	ticker := time.NewTicker(l.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.checkLastObserved(ctx)
		}
	}
}

func (l *lastObservedWatcher) checkLastObserved(ctx context.Context) {
	if !l.running.CompareAndSwap(false, true) {
		return
	}
	defer l.running.Store(false)

	result, err := l.alarmSvc.Stale(ctx)
	if err != nil {
		return
	}

	if result.TotalCount == 0 {
		return
	}

	for _, d := range result.Data {
		select {
		case <-ctx.Done():
			return
		default:
		}

		now := time.Now()
		desc := fmt.Sprintf("current time: %s, interval: %d, last seen: %s, limit: %s", now.UTC().Format(time.RFC3339), d.Interval, d.DeviceState.ObservedAt.Format(time.RFC3339), d.DeviceState.ObservedAt.Add(time.Duration(d.Interval)*time.Second).Format(time.RFC3339))

		l.alarmSvc.Add(ctx, d.DeviceID, types.AlarmDetails{
			AlarmType:   alarms.AlarmDeviceNotObserved,
			Description: desc,
			ObservedAt:  now.UTC(),
			Severity:    types.AlarmSeverityUnknown,
		})
	}
}
