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
)

func (dmc *devManagementClient) CreateDevice(ctx context.Context, device types.Device) error {
	var err error
	ctx, span := tracer.Start(ctx, "create-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	url := dmc.baseUrl + "/api/v0/devices"

	req, err := newJsonRequest(ctx, http.MethodPost, url, device)
	if err != nil {
		return err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to create device: %w", err)
		return err
	}
	defer drainAndCloseResponseBody(resp)

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}

	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusConflict {
			return ErrDeviceExist
		}
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return err
	}

	return nil
}

func (dmc *devManagementClient) FindDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error) {
	return dmc.findDeviceFromDevEUI(ctx, devEUI)
}

func (dmc *devManagementClient) findDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error) {
	var err error
	ctx, span := tracer.Start(ctx, "find-device-from-deveui")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx)
	log.Info("looking up internal id and types", "devEUI", devEUI)

	url := dmc.baseUrl + "/api/v0/devices?devEUI=" + devEUI

	req, err := newJsonRequest(ctx, http.MethodGet, url, nil)
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

	impls := struct {
		Data types.Device `json:"data"`
	}{}

	err = json.Unmarshal(respBody, &impls)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	device := impls.Data
	return &deviceWrapper{&device}, nil
}

func (dmc *devManagementClient) FindDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error) {
	return dmc.findDeviceFromInternalID(ctx, deviceID)
}

func (dmc *devManagementClient) findDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error) {
	var err error
	ctx, span := tracer.Start(ctx, "find-device-from-id")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx)
	log.Info("looking up properties for device", "device_id", deviceID)

	url := dmc.baseUrl + "/api/v0/devices/" + deviceID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve information for device: %w", err)
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

	impl := struct {
		Data types.Device `json:"data"`
	}{}

	err = json.Unmarshal(respBody, &impl)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	return &deviceWrapper{&impl.Data}, nil
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
	return d.impl.SensorProfile.Decoder
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
	return d.impl.Tenant
}

func (d *deviceWrapper) Source() string {
	return d.impl.Source
}
