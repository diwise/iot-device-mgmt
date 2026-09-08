package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/matryer/is"
)

// HARM-003: locks the device lifecycle event contracts published under
// these topic names. The events service subscribes to e.g.
// device.statusUpdated via its CloudEvents configuration, so topic names,
// content types and JSON field names must not change without a
// compatibility/migration plan and consumer-side contract tests.
func TestDeviceLifecycleEventContracts(t *testing.T) {
	is := is.New(t)

	ts := time.Date(2025, 4, 10, 11, 44, 1, 0, time.UTC)

	created := &DeviceCreated{DeviceID: "device-1", Tenant: "default", Timestamp: ts}
	is.Equal(created.TopicName(), "device.created")
	is.Equal(created.ContentType(), "application/json")

	updated := &DeviceUpdated{DeviceID: "device-1", Tenant: "default", Timestamp: ts}
	is.Equal(updated.TopicName(), "device.updated")
	is.Equal(updated.ContentType(), "application/json")

	statusUpdated := &DeviceStatusUpdated{DeviceID: "device-1", Tenant: "default", Timestamp: ts}
	is.Equal(statusUpdated.TopicName(), "device.statusUpdated")
	is.Equal(statusUpdated.ContentType(), "application/json")

	stateUpdated := &DeviceStateUpdated{DeviceID: "device-1", State: 1, Tenant: "default", Timestamp: ts}
	is.Equal(stateUpdated.TopicName(), "device.stateUpdated")
	is.Equal(stateUpdated.ContentType(), "application/json")

	bodies := [][]byte{created.Body(), updated.Body(), statusUpdated.Body(), stateUpdated.Body()}
	for _, body := range bodies {
		var decoded map[string]any
		is.NoErr(json.Unmarshal(body, &decoded))

		is.Equal(decoded["deviceID"], "device-1")
		is.Equal(decoded["tenant"], "default")

		_, ok := decoded["timestamp"]
		is.True(ok)
	}
}
