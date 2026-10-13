package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/matryer/is"
)

type recordingCloser struct {
	name  string
	calls *[]string
	n     int
}

func (f *recordingCloser) Close() {
	f.n++
	*f.calls = append(*f.calls, f.name)
}

type recordingWatchdog struct {
	calls *[]string
	n     int
}

func (f *recordingWatchdog) Start(context.Context) {}

func (f *recordingWatchdog) Stop(context.Context) {
	f.n++
	*f.calls = append(*f.calls, "watchdog")
}

// REV-005: shutdown must stop inflow, await admitted handlers, stop
// the watchdog and close storage exactly once, in messenger ->
// watchdog -> storage order, even when invoked twice. messenger.Close
// on a real context is not safe to call twice, hence the guard.
func TestShutdownIsOrderedAndIdempotent(t *testing.T) {
	is := is.New(t)

	var order []string
	wd := &recordingWatchdog{calls: &order}
	messenger := &messaging.MsgContextMock{
		ShutdownFunc: func(context.Context) error { order = append(order, "messenger"); return nil },
	}
	storage := &recordingCloser{name: "storage", calls: &order}

	owned := &ownedResources{watchdog: wd, messenger: messenger, tracker: &handlerTracker{}, storage: storage}

	ctx := context.Background()
	owned.close(ctx)
	owned.close(ctx)

	is.Equal(wd.n, 1)
	is.Equal(storage.n, 1)
	is.Equal(order, []string{"messenger", "watchdog", "storage"})
}

// REV-005: the watchdog must receive a real deadline even though the
// runner invokes OnShutdown without one.
func TestShutdownSuppliesWatchdogDeadline(t *testing.T) {
	is := is.New(t)

	var gotDeadline bool
	var hasDeadline bool
	wd := &deadlineRecordingWatchdog{
		onStop: func(ctx context.Context) {
			_, hasDeadline = ctx.Deadline()
			gotDeadline = true
		},
	}

	owned := &ownedResources{watchdog: wd}
	owned.close(context.Background())

	is.True(gotDeadline)
	is.True(hasDeadline)
}

type deadlineRecordingWatchdog struct {
	onStop func(context.Context)
}

func (f *deadlineRecordingWatchdog) Start(context.Context) {}

func (f *deadlineRecordingWatchdog) Stop(ctx context.Context) {
	f.onStop(ctx)
}

// REV-005: tracked handlers are awaited within budget; wait reports
// whether all admitted deliveries finished.
func TestHandlerTrackerWaitsForInflight(t *testing.T) {
	is := is.New(t)

	tracker := &handlerTracker{}
	release := make(chan struct{})
	handlerStarted := make(chan struct{})

	tracked := tracker.track(func(context.Context, messaging.IncomingTopicMessage, *slog.Logger) error {
		close(handlerStarted)
		<-release
		return nil
	})

	done := make(chan bool, 1)
	go func() {
		tracked(context.Background(), nil, slog.Default())
	}()

	<-handlerStarted
	go func() { done <- tracker.wait(5 * time.Second) }()

	select {
	case <-done:
		t.Fatal("wait returned while handler still blocked")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	is.True(<-done)
}

// REV-005: wait times out instead of hanging shutdown forever.
func TestHandlerTrackerWaitTimesOut(t *testing.T) {
	is := is.New(t)

	tracker := &handlerTracker{}
	tracker.wg.Add(1)
	defer tracker.wg.Done()

	is.True(!tracker.wait(20 * time.Millisecond))
}

// REV-005: messaging configuration panics must surface as errors, not
// crash startup after storage was opened.
func TestInitMessengerConvertsPanicToError(t *testing.T) {
	is := is.New(t)

	t.Setenv("RABBITMQ_HOST", "")
	t.Setenv("RABBITMQ_DISABLED", "false")

	messenger, err := initMessenger(context.Background(), serviceName, slog.Default())
	is.True(err != nil)
	is.True(messenger == nil)
}

// BASE-003: shutdown with no initialized resources (e.g. failed OnInit)
// must be a safe no-op.
func TestShutdownWithoutResourcesIsSafe(t *testing.T) {
	owned := &ownedResources{}

	owned.close(context.Background())
	owned.close(context.Background())
}

// BASE-004: readiness stubs always report OK without touching any
// dependency, even when fakes report failures.
func TestReadinessStubsAlwaysOK(t *testing.T) {
	is := is.New(t)

	probes := readinessProbes()
	is.Equal(len(probes), 2)

	for _, name := range []string{"rabbitmq", "timescale"} {
		status, err := probes[name](context.Background())
		is.NoErr(err)
		is.Equal(status, "ok")
	}
}
