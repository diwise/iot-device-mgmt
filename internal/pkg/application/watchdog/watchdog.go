package watchdog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
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
	done     chan bool
	alarmSvc alarms.AlarmService
	config   *WatchdogConfig
}

func New(a alarms.AlarmService, cfg *WatchdogConfig) Watchdog {
	w := &watchdogImpl{
		done:     make(chan bool),
		alarmSvc: a,
		config:   cfg,
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
		alarmSvc: w.alarmSvc,
		running:  false,
		interval: 10 * time.Minute,
	}

	if w.config.Interval > 0 {
		l.interval = time.Duration(w.config.Interval) * time.Minute
	}

	go l.Watch(ctx)

	for {
		select {
		case <-w.done:
			return
		case <-ctx.Done():
			return
		}
	}
}

type Watcher interface {
	Watch(ctx context.Context)
}

type lastObservedWatcher struct {
	alarmSvc alarms.AlarmService
	running  bool
	interval time.Duration
	mu       sync.Mutex
}

func (l *lastObservedWatcher) Watch(ctx context.Context) {
	ticker := time.NewTicker(l.interval)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go l.checkLastObserved(ctx)
		}
	}
}

func (l *lastObservedWatcher) setRunning(b bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.running = b
}

func (l *lastObservedWatcher) checkLastObserved(ctx context.Context) {
	if l.running {
		return
	}

	l.setRunning(true)

	result, err := l.alarmSvc.GetStaleDevices(ctx)
	if err != nil {
		l.setRunning(false)
		return
	}

	if result.TotalCount == 0 {
		l.setRunning(false)
		return
	}

	for _, d := range result.Data {

		now := time.Now()
		desc := fmt.Sprintf("current time: %s, interval: %d, last seen: %s, limit: %s", now.UTC().Format(time.RFC3339), d.Interval, d.DeviceState.ObservedAt.Format(time.RFC3339), d.DeviceState.ObservedAt.Add(time.Duration(d.Interval)*time.Second).Format(time.RFC3339))

		l.alarmSvc.Add(ctx, d.DeviceID, types.AlarmDetails{
			AlarmType:   alarms.AlarmDeviceNotObserved,
			Description: desc,
			ObservedAt:  now.UTC(),
			Severity:    types.AlarmSeverityUnknown,
		})
	}

	l.setRunning(false)
}
