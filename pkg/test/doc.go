// Package client (import path pkg/test) provides generated mocks of the
// public client interfaces for external consumers' tests.
//
// The mocks mirror pkg/client and pkg/types one-to-one and are
// imported by (among others) iot-agent and iot-core test suites.
// Regenerate with moq after any interface change and verify consumers
// before release, following the same compatible release sequence as
// the packages they mock.
package client
