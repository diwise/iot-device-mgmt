package watchdog

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type blockingWatcher struct {
	started  chan struct{}
	stopped  chan struct{}
	watching atomic.Bool
}

func (w *blockingWatcher) Watch(ctx context.Context) {
	w.watching.Store(true)
	close(w.started)
	<-ctx.Done()
	w.watching.Store(false)
	close(w.stopped)
}

func TestWatchdogStopStopsBackgroundActivity(t *testing.T) {
	watcher := &blockingWatcher{
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}

	wd := &watchdogImpl{
		watchers: []Watcher{watcher},
		done:     make(chan struct{}),
	}

	wd.Start(context.Background())

	select {
	case <-watcher.started:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not start")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wd.Stop(stopCtx)

	select {
	case <-watcher.stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop")
	}

	if watcher.watching.Load() {
		t.Fatal("expected watcher to stop all background activity")
	}
	if wd.running.Load() {
		t.Fatal("expected watchdog to be marked stopped")
	}
}
