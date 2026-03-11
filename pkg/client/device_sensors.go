package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func (dmc *devManagementClient) AttachSensorToDevice(ctx context.Context, deviceID, sensorID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "attach-sensor-to-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	url := dmc.baseUrl + "/api/v0/devices/" + deviceID + "/sensor"

	req, err := newJsonRequest(ctx, http.MethodPut, url, map[string]string{"sensorID": sensorID})
	if err != nil {
		return err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to attach sensor to device: %w", err)
	}
	defer drainAndCloseResponseBody(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	case http.StatusUnauthorized:
		return ErrUnauthorized
	default:
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}
}

func (dmc *devManagementClient) DetachSensorFromDevice(ctx context.Context, deviceID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "detach-sensor-from-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	url := dmc.baseUrl + "/api/v0/devices/" + deviceID + "/sensor"

	req, err := newJsonRequest(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to detach sensor from device: %w", err)
	}
	defer drainAndCloseResponseBody(resp)

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}
}
