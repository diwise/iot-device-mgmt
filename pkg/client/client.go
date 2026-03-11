package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

//go:generate moq -rm -out ../test/client_mock.go . DeviceManagementClient

type DeviceManagementClient interface {
	Close(ctx context.Context)
	CreateDevice(ctx context.Context, device types.Device) error
	FindDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error)
	FindDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error)
	CreateSensor(ctx context.Context, sensor types.SensorConfig) error
	UpdateSensor(ctx context.Context, sensor types.SensorConfig) error
	GetSensor(ctx context.Context, sensorID string) (Sensor, error)
	ListSensors(ctx context.Context, query types.SensorsQuery) ([]Sensor, error)
	AttachSensorToDevice(ctx context.Context, deviceID, sensorID string) error
	DetachSensorFromDevice(ctx context.Context, deviceID string) error
	GetTenants(ctx context.Context) ([]string, error)
	GetDeviceProfiles(ctx context.Context) ([]types.SensorProfile, error)
	GetDeviceProfile(ctx context.Context, deviceProfileID string) (*types.SensorProfile, error)
}

/*

type deviceState int

const (
	Refreshing deviceState = iota
	Ready
	Error
)

type devEUIState struct {
	state      deviceState
	err        error
	internalID string
	when       time.Time
}

type lookupResult struct {
	state  deviceState
	device Device
	err    error
	when   time.Time
}
*/

type devManagementClient struct {
	baseUrl           string
	clientCredentials *clientcredentials.Config
	insecureURL       bool

	queue       chan (func())
	httpClient  http.Client
	debugClient bool

	//errorCacheTTL     time.Duration
	//cacheByInternalID map[string]lookupResult
	//knownDevEUI       map[string]devEUIState

	keepRunning *atomic.Bool

	wg sync.WaitGroup
}

var tracer = otel.Tracer("device-mgmt-client")

func New(ctx context.Context, baseUrl, oauthTokenURL string, oauthInsecureURL bool, oauthClientID, oauthClientSecret string) (DeviceManagementClient, error) {

	// configure transport
	baseTransport := http.DefaultTransport.(*http.Transport).Clone()
	baseTransport.MaxIdleConns = 100
	baseTransport.MaxIdleConnsPerHost = 20
	baseTransport.IdleConnTimeout = 90 * time.Second

	// skip TLS verification if configured (e.g. for local testing with self-signed certs)
	if oauthInsecureURL {
		baseTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	// client for OAuth tokens
	oauthClient := &http.Client{
		Transport: otelhttp.NewTransport(baseTransport),
		Timeout:   10 * time.Second,
	}

	// Create OAuth context that will be reused for all token operations
	oauthCtx := context.WithValue(context.Background(), oauth2.HTTPClient, oauthClient)

	oauthConfig := &clientcredentials.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		TokenURL:     oauthTokenURL,
	}

	// maby we should use oauthConfig.Client(oauthCtx) instead of creating our own transport?
	ts := oauthConfig.TokenSource(oauthCtx)

	// ts only to be able to wrap the transport with otelhttp
	apiTransport := &oauth2.Transport{
		Source: ts,
		Base:   otelhttp.NewTransport(baseTransport),
	}

	// client for API requests, using the OAuth transport to automatically add tokens and refresh them as needed
	apiClient := &http.Client{
		Transport: apiTransport,
		Timeout:   30 * time.Second,
	}

	// fail fast if token cannot be retrieved with the provided credentials
	token, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get client credentials from %s: %w", oauthConfig.TokenURL, err)
	}

	if !token.Valid() {
		return nil, fmt.Errorf("an invalid token was returned from %s", oauthTokenURL)
	}

	dmc := &devManagementClient{
		baseUrl:           baseUrl,
		clientCredentials: oauthConfig,
		insecureURL:       oauthInsecureURL,

		httpClient:  *apiClient,
		debugClient: env.GetVariableOrDefault(ctx, "DEVMGMT_CLIENT_DEBUG", "false") == "true",

		//cacheByInternalID: make(map[string]lookupResult, 100),
		//errorCacheTTL:     30 * time.Second,
		//knownDevEUI:       make(map[string]devEUIState, 100),

		queue:       make(chan func()),
		keepRunning: &atomic.Bool{},
	}
	// TODO: with cache removed, this is not really needed anymore,
	// but we might want to keep it for future use if we want to add background tasks or caching back in
	go dmc.run(ctx)

	return dmc, nil
}

var ErrDeviceExist = errors.New("device already exists")
var ErrConflict = errors.New("conflict")
var ErrUnauthorized = errors.New("not authorized")
var ErrNotFound error = errors.New("not found")

func (dmc *devManagementClient) run(ctx context.Context) {
	dmc.wg.Add(1)
	defer dmc.wg.Done()

	logger := logging.GetFromContext(ctx)
	logger.Info("starting up device management client")

	// use atomic swap to avoid startup races
	alreadyStarted := dmc.keepRunning.Swap(true)
	if alreadyStarted {
		logger.Error("attempt to start the device management client multiple times")
		return
	}

	for dmc.keepRunning.Load() {
		fn := <-dmc.queue
		fn()
	}

	logger.Info("device management client exiting")
}

func (dmc *devManagementClient) Close(ctx context.Context) {
	dmc.queue <- func() {
		dmc.keepRunning.Store(false)
	}

	dmc.wg.Wait()
}

func drainAndCloseResponseBody(r *http.Response) {
	defer r.Body.Close()
	io.Copy(io.Discard, r.Body)
}

func (dmc *devManagementClient) dumpRequestResponseIfNon200AndDebugEnabled(ctx context.Context, req *http.Request, resp *http.Response) {
	if dmc.debugClient && (resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusNotFound) {
		reqbytes, _ := httputil.DumpRequest(req, false)
		respbytes, _ := httputil.DumpResponse(resp, false)

		log := logging.GetFromContext(ctx)
		log.Debug("request failed", "request", string(reqbytes), "response", string(respbytes))
	}
}

func newJsonRequest(ctx context.Context, method, url string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}
	if body != nil {
		req.Header.Add("Content-Type", "application/json")
	}

	return req, nil
}
