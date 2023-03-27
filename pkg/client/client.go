package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2/clientcredentials"
)

type DeviceManagementClient interface {
	FindDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error)
	FindDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error)
}

type devManagementClient struct {
	url               string
	clientCredentials *clientcredentials.Config
}

var tracer = otel.Tracer("device-mgmt-client")

func New(ctx context.Context, devMgmtUrl, oauthTokenURL, oauthClientID, oauthClientSecret string) (DeviceManagementClient, error) {
	oauthConfig := &clientcredentials.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		TokenURL:     oauthTokenURL,
	}

	token, err := oauthConfig.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get client credentials from %s: %w", oauthConfig.TokenURL, err)
	}

	if !token.Valid() {
		return nil, fmt.Errorf("an invalid token was returned from %s", oauthTokenURL)
	}

	dmc := &devManagementClient{
		url:               devMgmtUrl,
		clientCredentials: oauthConfig,
	}

	return dmc, nil
}

func (dmc *devManagementClient) FindDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error) {
	var err error
	ctx, span := tracer.Start(ctx, "find-device-from-deveui")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx)
	log.Info().Msgf("looking up internal id and types for devEUI %s", devEUI)

	httpClient := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	url := dmc.url + "/api/v0/devices?devEUI=" + devEUI

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	if dmc.clientCredentials != nil {
		token, err := dmc.clientCredentials.Token(ctx)
		if err != nil {
			err = fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve device information from devEUI: %w", err)
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
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

	impls := []types.Device{}

	err = json.Unmarshal(respBody, &impls)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	if len(impls) == 0 {
		err = fmt.Errorf("device management returned an empty list of devices")
		return nil, err
	}

	device := impls[0]
	return &deviceWrapper{&device}, nil
}

func (dmc *devManagementClient) FindDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error) {
	var err error
	ctx, span := tracer.Start(ctx, "find-device-from-id")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx)
	log.Info().Msgf("looking up properties for device %s", deviceID)

	httpClient := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	url := dmc.url + "/api/v0/devices/" + deviceID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	if dmc.clientCredentials != nil {
		token, err := dmc.clientCredentials.Token(ctx)
		if err != nil {
			err = fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve information for device: %w", err)
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
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

	impl := &types.Device{}

	err = json.Unmarshal(respBody, impl)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	return &deviceWrapper{impl}, nil
}

//go:generate moq -rm -out ../test/device_mock.go . Device

// you need to modify the generated ../test/device_mock.go
// import "github.com/diwise/iot-device-mgmt/pkg/client"
// change "var _ Device = &DeviceMock{}" to "var _ client.Device = &DeviceMock{}"
type Device interface {
	ID() string
	Latitude() float64
	Longitude() float64
	Environment() string
	Types() []string
	SensorType() string
	IsActive() bool
	Tenant() string
}

type deviceWrapper struct {
	impl *types.Device
}

func (d *deviceWrapper) ID() string {
	return d.impl.DeviceID
}

func (d *deviceWrapper) Latitude() float64 {
	return d.impl.Location.Latitude
}

func (d *deviceWrapper) Longitude() float64 {
	return d.impl.Location.Longitude
}

func (d *deviceWrapper) Environment() string {
	return d.impl.Environment
}

func (d *deviceWrapper) SensorType() string {
	return d.impl.DeviceProfile.Decoder
}

func (d *deviceWrapper) Types() []string {
	return []string{}
}

func (d *deviceWrapper) IsActive() bool {
	return d.impl.Active
}

func (d *deviceWrapper) Tenant() string {
	return d.impl.Tenant.Name
}
