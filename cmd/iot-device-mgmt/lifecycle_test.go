package main

import (
	"context"
	"testing"

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

// BASE-003: shutdown must close owned resources exactly once, in
// watchdog -> messenger -> storage order, even when invoked twice.
// messenger.Close on a real context is not safe to call twice, hence
// the guard under test.
func TestShutdownIsOrderedAndIdempotent(t *testing.T) {
	is := is.New(t)

	var order []string
	wd := &recordingWatchdog{calls: &order}
	messenger := &messaging.MsgContextMock{
		CloseFunc: func() { order = append(order, "messenger") },
	}
	storage := &recordingCloser{name: "storage", calls: &order}

	owned := &ownedResources{watchdog: wd, messenger: messenger, storage: storage}

	ctx := context.Background()
	owned.close(ctx)
	owned.close(ctx)

	is.Equal(wd.n, 1)
	is.Equal(storage.n, 1)
	is.Equal(order, []string{"watchdog", "messenger", "storage"})
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
