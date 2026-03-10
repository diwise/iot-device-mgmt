package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func (dmc *devManagementClient) GetDeviceProfiles(ctx context.Context) ([]types.SensorProfile, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-device-profiles")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	url := dmc.url + "/api/v0/admin/deviceprofiles"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := dmc.doRequestWithTokenRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	response := struct {
		Data []types.SensorProfile `json:"data"`
	}{}
	err = json.Unmarshal(body, &response)
	if err == nil && len(response.Data) > 0 {
		return response.Data, nil
	}

	single := struct {
		Data types.SensorProfile `json:"data"`
	}{}
	err = json.Unmarshal(body, &single)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return []types.SensorProfile{single.Data}, nil
}

func (dmc *devManagementClient) GetTenants(ctx context.Context) ([]string, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-tenants")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	url := dmc.url + "/api/v0/admin/tenants"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := dmc.doRequestWithTokenRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	response := struct {
		Data []string `json:"data"`
	}{}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return response.Data, nil
}
