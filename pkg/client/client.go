package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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

type lookupResult struct {
	device Device
	err    error
	when   time.Time
}

type devManagementClient struct {
	url               string
	clientCredentials *clientcredentials.Config

	cache map[string]lookupResult
	inbox chan (func())
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

		cache: make(map[string]lookupResult, 100),
		inbox: make(chan func()),
	}

	go func() {
		for f := range dmc.inbox {
			f()
		}
	}()

	return dmc, nil
}

func (dmc *devManagementClient) FindDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error) {
	resultchan := make(chan Device)
	errchan := make(chan error)

	dmc.inbox <- func() {
		r, ok := dmc.cache[devEUI]

		if !ok {
			d, err := dmc.findDeviceFromDevEUI(ctx, devEUI)
			r = lookupResult{device: d, err: err, when: time.Now()}
			dmc.cache[devEUI] = r
			dmc.cache[d.ID()] = r
		}

		if r.err != nil {
			errchan <- r.err
			return
		}

		resultchan <- r.device
	}

	select {
	case d := <-resultchan:
		return d, nil
	case e := <-errchan:
		return nil, e
	}
}

func (dmc *devManagementClient) findDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error) {
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
	defer resp.Body.Close()

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
	resultchan := make(chan Device)
	errchan := make(chan error)

	dmc.inbox <- func() {
		r, ok := dmc.cache[deviceID]

		if !ok {
			d, err := dmc.findDeviceFromInternalID(ctx, deviceID)
			r = lookupResult{device: d, err: err, when: time.Now()}
			dmc.cache[deviceID] = r
		}

		if r.err != nil {
			errchan <- r.err
			return
		}

		resultchan <- r.device
	}

	select {
	case d := <-resultchan:
		return d, nil
	case e := <-errchan:
		return nil, e
	}
}

func (dmc *devManagementClient) findDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error) {
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
	defer resp.Body.Close()

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
	Environment() string
	IsActive() bool
	Latitude() float64
	Longitude() float64
	SensorType() string
	Source() string
	Tenant() string
	Types() []string
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
	types := []string{}
	for _, t := range d.impl.Lwm2mTypes {
		types = append(types, t.Urn)
	}
	return types
}

func (d *deviceWrapper) IsActive() bool {
	return d.impl.Active
}

func (d *deviceWrapper) Tenant() string {
	return d.impl.Tenant.Name
}

func (d *deviceWrapper) Source() string {
	return d.impl.Source
}
