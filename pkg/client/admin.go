package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func (dmc *devManagementClient) GetDeviceProfile(ctx context.Context, deviceProfileID string) (*types.SensorProfile, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-device-profile")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("name", deviceProfileID)

	req, err := newJsonRequest(ctx, http.MethodGet, dmc.baseUrl+"/api/v0/admin/deviceprofiles?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve device information from devEUI: %w", err)
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return nil, err
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return nil, err
	}

	responseData := struct {
		Data types.SensorProfile `json:"data"`
	}{}

	err = json.Unmarshal(respBody, &responseData)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	deviceprofile := responseData.Data

	return &deviceprofile, nil
}

func (dmc *devManagementClient) GetDeviceProfiles(ctx context.Context) ([]types.SensorProfile, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-device-profiles")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	req, err := newJsonRequest(ctx, http.MethodGet, dmc.baseUrl+"/api/v0/admin/deviceprofiles", nil)
	if err != nil {
		return nil, err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

	if resp.StatusCode != http.StatusOK {
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

	req, err := newJsonRequest(ctx, http.MethodGet, dmc.baseUrl+"/api/v0/admin/tenants", nil)
	if err != nil {
		return nil, err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}

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
