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

// NOTE: the four types above are defined as public contracts, but no
// service currently publishes them (verified: no PublishOnTopic call in
// any iot-* service produces device.created/updated/statusUpdated/
// stateUpdated). Consumers must not rely on receiving these events
// until a producer is implemented with a compatibility plan.

// REV-016: locks the complete wire representation of every lifecycle
// event with a non-default tenant and fixed measurement time, including
// the state field and nested status payload.
func TestDeviceLifecycleEventGoldenBodies(t *testing.T) {
	is := is.New(t)

	ts := time.Date(2025, 4, 10, 11, 44, 1, 0, time.UTC)
	rssi := -110.0

	created := &DeviceCreated{DeviceID: "device-1", Tenant: "acme", Timestamp: ts}
	is.Equal(string(created.Body()), `{"deviceID":"device-1","tenant":"acme","timestamp":"2025-04-10T11:44:01Z"}`)

	updated := &DeviceUpdated{DeviceID: "device-1", Tenant: "acme", Timestamp: ts}
	is.Equal(string(updated.Body()), `{"deviceID":"device-1","tenant":"acme","timestamp":"2025-04-10T11:44:01Z"}`)

	statusUpdated := &DeviceStatusUpdated{
		DeviceID:     "device-1",
		DeviceStatus: SensorStatus{BatteryLevel: 87, RSSI: &rssi, ObservedAt: ts},
		Tenant:       "acme",
		Timestamp:    ts,
	}
	is.Equal(string(statusUpdated.Body()), `{"deviceID":"device-1","status":{"batteryLevel":87,"rssi":-110,"observedAt":"2025-04-10T11:44:01Z"},"tenant":"acme","timestamp":"2025-04-10T11:44:01Z"}`)

	stateUpdated := &DeviceStateUpdated{DeviceID: "device-1", State: 2, Tenant: "acme", Timestamp: ts}
	is.Equal(string(stateUpdated.Body()), `{"deviceID":"device-1","state":2,"tenant":"acme","timestamp":"2025-04-10T11:44:01Z"}`)

	// The state value round-trips; a dropped or renamed state field
	// fails here, not just in key-existence checks.
	var decodedState map[string]any
	is.NoErr(json.Unmarshal(stateUpdated.Body(), &decodedState))
	is.Equal(decodedState["state"], 2.0)
}
