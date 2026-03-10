package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func (dmc *devManagementClient) AttachSensorToDevice(ctx context.Context, deviceID, sensorID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "attach-sensor-to-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	requestBody, err := json.Marshal(map[string]string{"sensorID": sensorID})
	if err != nil {
		return fmt.Errorf("failed to marshal sensor assignment: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, dmc.url+"/api/v0/devices/"+deviceID+"/sensor", bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := dmc.doRequestWithTokenRetry(ctx, req)
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
	default:
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}
}

func (dmc *devManagementClient) DetachSensorFromDevice(ctx context.Context, deviceID string) error {
	var err error
	ctx, span := tracer.Start(ctx, "detach-sensor-from-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, dmc.url+"/api/v0/devices/"+deviceID+"/sensor", nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := dmc.doRequestWithTokenRetry(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to detach sensor from device: %w", err)
	}
	defer drainAndCloseResponseBody(resp)

	switch resp.StatusCode {
	case http.StatusNoContent:
		return nil
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}
}
