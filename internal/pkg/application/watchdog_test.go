package application

import (
	"testing"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/matryer/is"
)

func TestSecondUntil(t *testing.T) {
	is := is.New(t)
	ti, _ := time.Parse(time.RFC3339, "2022-01-01T00:00:00Z")
	now, _ := time.Parse(time.RFC3339, "2022-01-01T00:00:00Z")

	s := timeToNextTime(types.Device{LastObserved: ti, Intervall: 10}, now)

	is.Equal(10, s)
}
