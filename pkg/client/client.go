package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
)

type DeviceManagementClient interface {
	FindDeviceFromDevEUI(ctx context.Context, devEUI string) (*QueryResult, error)
	FindDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error)
}

type devManagementClient struct {
	url string
}

var tracer = otel.Tracer("device-mgmt-client")

func NewDeviceManagementClient(devMgmtUrl string) DeviceManagementClient {
	dmc := &devManagementClient{
		url: devMgmtUrl,
	}
	return dmc
}

func (dmc *devManagementClient) FindDeviceFromDevEUI(ctx context.Context, devEUI string) (*QueryResult, error) {
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

	resp, err := httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve device information from devEUI: %w", err)
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("request failed, no device found")
		return nil, err
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return nil, err
	}

	result := []QueryResult{}

	err = json.Unmarshal(respBody, &result)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	if len(result) == 0 {
		err = fmt.Errorf("device management returned an empty list of devices")
		return nil, err
	}

	device := result[0]
	return &device, nil
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
		log.Error().Err(err).Msg("failed to create http request")
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Error().Msgf("failed to retrieve information for device: %s", err.Error())
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		log.Error().Msgf("request failed with status code %d", resp.StatusCode)
		return nil, fmt.Errorf("request failed, no device found")
	}

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error().Msgf("failed to read response body: %s", err.Error())
		return nil, err
	}

	result := &device{}

	err = json.Unmarshal(respBody, result)
	if err != nil {
		log.Error().Msgf("failed to unmarshal response body: %s", err.Error())
		return nil, err
	}

	return result, nil
}

type Device interface {
	ID() string
	Latitude() float64
	Longitude() float64
	Environment() string
	Types() []string
}

type device struct {
	Identity string   `json:"id"`
	Lat      float64  `json:"latitude"`
	Long     float64  `json:"longitude"`
	Env      string   `json:"environment"`
	Types_   []string `json:"types"`
}

func NewDevice(id, env string, lat, long float64) Device {
	return &device{
		Identity: id,
		Env:      env,
		Lat:      lat,
		Long:     long,
	}
}

func (d *device) ID() string {
	return d.Identity
}

func (d *device) Latitude() float64 {
	return d.Lat
}

func (d *device) Longitude() float64 {
	return d.Long
}

func (d *device) Environment() string {
	return d.Env
}

func (d *device) Types() []string {
	return d.Types_
}

type QueryResult struct {
	InternalID string   `json:"id"`
	SensorType string   `json:"sensorType"`
	Types      []string `json:"types"`
	IsActive   bool     `json:"active"`
}
