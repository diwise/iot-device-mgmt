// Package types defines the stable public data contracts of
// iot-device-management.
//
// These types are consumed outside this module, notably by iot-agent
// (device lookup/creation, sensor models, event envelopes) and
// iot-core. Field names, JSON tags and envelope topics/content types
// must not change without a compatibility/migration plan and
// contract tests on the consumer side.
//
// Moving, renaming or deleting anything in this package requires a
// compatible release sequence: release iot-device-mgmt first, then
// upgrade and verify each consumer before removing old API.
package types
