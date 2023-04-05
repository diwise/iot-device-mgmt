package watchdog

import (
	"testing"
	"time"

	"github.com/matryer/is"
	"github.com/rs/zerolog"
)

func TestCheckLastObserved(t *testing.T) {
	is := is.New(t)

	observed, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	is.NoErr(err)
	now, err := time.Parse(time.RFC3339, "2006-01-02T15:03:54Z")
	is.NoErr(err)

	is.True(checkLastObservedIsAfter(zerolog.Logger{}, observed, now, 10))

	now = now.Add(30 * time.Second)
	is.True(!checkLastObservedIsAfter(zerolog.Logger{}, observed, now, 10))
}
