package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

type Sensor interface {
	ID() string
	DeviceID() string
	IsAssigned() bool
	SensorType() string
	ProfileName() string
	Interval() int
}

type sensorWrapper struct {
	impl *types.Sensor
}

func (s *sensorWrapper) ID() string {
	return s.impl.SensorID
}

func (s *sensorWrapper) DeviceID() string {
	if s.impl.DeviceID == nil {
		return ""
	}
	return *s.impl.DeviceID
}

func (s *sensorWrapper) IsAssigned() bool {
	return s.impl.DeviceID != nil && *s.impl.DeviceID != ""
}

func (s *sensorWrapper) SensorType() string {
	if s.impl.SensorProfile == nil {
		return ""
	}
	return s.impl.SensorProfile.Decoder
}

func (s *sensorWrapper) ProfileName() string {
	if s.impl.SensorProfile == nil {
		return ""
	}
	return s.impl.SensorProfile.Name
}

func (s *sensorWrapper) Interval() int {
	if s.impl.SensorProfile == nil {
		return 0
	}
	return s.impl.SensorProfile.Interval
}

func (dmc *devManagementClient) GetSensor(ctx context.Context, sensorID string) (Sensor, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	url := dmc.baseUrl + "/api/v0/sensors/" + sensorID

	req, err := newJsonRequest(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve sensor: %w", err)
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
		Data types.Sensor `json:"data"`
	}{}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return &sensorWrapper{impl: &response.Data}, nil
}

func (dmc *devManagementClient) ListSensors(ctx context.Context, query types.SensorsQuery) ([]Sensor, error) {
	var err error
	ctx, span := tracer.Start(ctx, "list-sensors")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	if query.Offset != nil {
		params.Set("offset", strconv.Itoa(*query.Offset))
	}
	if query.Limit != nil {
		params.Set("limit", strconv.Itoa(*query.Limit))
	}
	if query.Assigned != nil {
		params.Set("assigned", strconv.FormatBool(*query.Assigned))
	}
	if query.HasProfile != nil {
		params.Set("hasProfile", strconv.FormatBool(*query.HasProfile))
	}
	if profileName := strings.TrimSpace(query.ProfileName); profileName != "" {
		params.Set("profileName", profileName)
	}
	if len(query.Types) > 0 {
		params["types"] = append([]string(nil), query.Types...)
	}

	requestURL := dmc.baseUrl + "/api/v0/sensors"
	if encoded := params.Encode(); encoded != "" {
		requestURL += "?" + encoded
	}

	req, err := newJsonRequest(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query sensors: %w", err)
	}
	defer drainAndCloseResponseBody(resp)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed with status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	response := struct {
		Data []types.Sensor `json:"data"`
	}{}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	sensors := make([]Sensor, 0, len(response.Data))
	for i := range response.Data {
		record := response.Data[i]
		sensors = append(sensors, &sensorWrapper{impl: &record})
	}

	return sensors, nil
}

func (dmc *devManagementClient) CreateSensor(ctx context.Context, sensor types.SensorConfig) error {
	return dmc.writeSensor(ctx, http.MethodPost, dmc.baseUrl+"/api/v0/sensors", sensor)
}

func (dmc *devManagementClient) UpdateSensor(ctx context.Context, sensor types.SensorConfig) error {
	return dmc.writeSensor(ctx, http.MethodPut, dmc.baseUrl+"/api/v0/sensors/"+sensor.SensorID, sensor)
}

func (dmc *devManagementClient) writeSensor(ctx context.Context, method, requestURL string, sensor types.SensorConfig) error {
	var err error
	ctx, span := tracer.Start(ctx, "write-sensor")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	payload := struct {
		SensorID      string               `json:"sensorID"`
		Name          *string              `json:"name,omitempty"`
		Location      *types.Location      `json:"location,omitempty"`
		SensorProfile *types.SensorProfile `json:"sensorProfile,omitempty"`
	}{
		SensorID: sensor.SensorID,
		Name:     sensor.Name,
		Location: sensor.Location,
	}
	if sensor.SensorProfileID != "" {
		payload.SensorProfile = &types.SensorProfile{Decoder: sensor.SensorProfileID}
	}

	req, err := newJsonRequest(ctx, method, requestURL, payload)
	if err != nil {
		return err
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to write sensor: %w", err)
	}
	defer drainAndCloseResponseBody(resp)

	switch resp.StatusCode {
	case http.StatusCreated, http.StatusOK:
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
