// Package client is the stable public HTTP client for
// iot-device-management.
//
// It is consumed outside this module, notably by iot-agent and
// iot-core. Method signatures, request/response formats and client
// behavior must not change without a compatibility/migration plan and
// verification against each consumer.
//
// Moving, renaming or deleting anything in this package requires a
// compatible release sequence: release iot-device-mgmt first, then
// upgrade and verify each consumer before removing old API.
package client
