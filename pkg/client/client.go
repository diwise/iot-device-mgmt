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
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

//go:generate moq -rm -out ../test/client_mock.go . DeviceManagementClient

type DeviceManagementClient interface {
	FindDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error)
	FindDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error)
	Close(ctx context.Context)
	CreateDevice(ctx context.Context, device types.Device) error
	GetDeviceProfile(ctx context.Context, deviceProfileID string) (*types.DeviceProfile, error)
}

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

type devManagementClient struct {
	url               string
	clientCredentials *clientcredentials.Config
	insecureURL       bool

	cacheByInternalID map[string]lookupResult
	knownDevEUI       map[string]devEUIState
	queue             chan (func())
	httpClient        http.Client
	debugClient       bool
	errorCacheTTL     time.Duration

	// OAuth token management
	oauthCtx    context.Context
	cachedToken *oauth2.Token
	tokenMutex  sync.RWMutex

	keepRunning *atomic.Bool
	wg          sync.WaitGroup
}

var tracer = otel.Tracer("device-mgmt-client")

func New(ctx context.Context, devMgmtUrl, oauthTokenURL string, oauthInsecureURL bool, oauthClientID, oauthClientSecret string) (DeviceManagementClient, error) {
	oauthConfig := &clientcredentials.Config{
		ClientID:     oauthClientID,
		ClientSecret: oauthClientSecret,
		TokenURL:     oauthTokenURL,
	}

	httpTransport := http.DefaultTransport
	if oauthInsecureURL {
		trans, ok := httpTransport.(*http.Transport)
		if ok {
			if trans.TLSClientConfig == nil {
				trans.TLSClientConfig = &tls.Config{}
			}
			trans.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	httpClient := &http.Client{
		Transport: otelhttp.NewTransport(httpTransport),
	}

	// Create OAuth context that will be reused for all token operations
	oauthCtx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	token, err := oauthConfig.Token(oauthCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get client credentials from %s: %w", oauthConfig.TokenURL, err)
	}

	if !token.Valid() {
		return nil, fmt.Errorf("an invalid token was returned from %s", oauthTokenURL)
	}

	dmc := &devManagementClient{
		url:               devMgmtUrl,
		clientCredentials: oauthConfig,
		insecureURL:       oauthInsecureURL,

		cacheByInternalID: make(map[string]lookupResult, 100),
		knownDevEUI:       make(map[string]devEUIState, 100),
		queue:             make(chan func()),
		keepRunning:       &atomic.Bool{},

		httpClient:    *httpClient,
		debugClient:   env.GetVariableOrDefault(ctx, "DEVMGMT_CLIENT_DEBUG", "false") == "true",
		errorCacheTTL: 30 * time.Second,

		oauthCtx:    oauthCtx,
		cachedToken: token,
	}

	go dmc.run(ctx)

	return dmc, nil
}

var ErrDeviceExist = errors.New("device already exists")

func drainAndCloseResponseBody(r *http.Response) {
	defer r.Body.Close()
	io.Copy(io.Discard, r.Body)
}

// invalidateTokenCache clears the cached token to force a refresh on next request
func (dmc *devManagementClient) invalidateTokenCache() {
	dmc.tokenMutex.Lock()
	defer dmc.tokenMutex.Unlock()
	dmc.cachedToken = nil
}

func (dmc *devManagementClient) dumpRequestResponseIfNon200AndDebugEnabled(ctx context.Context, req *http.Request, resp *http.Response) {
	if dmc.debugClient && (resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusNotFound) {
		reqbytes, _ := httputil.DumpRequest(req, false)
		respbytes, _ := httputil.DumpResponse(resp, false)

		log := logging.GetFromContext(ctx)
		log.Debug("request failed", "request", string(reqbytes), "response", string(respbytes))
	}
}

func (dmc *devManagementClient) refreshToken(ctx context.Context) (token *oauth2.Token, err error) {
	ctx, span := tracer.Start(ctx, "refresh-token")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	// Check if cached token is still valid
	dmc.tokenMutex.RLock()
	if dmc.cachedToken != nil && dmc.cachedToken.Valid() {
		token = dmc.cachedToken
		dmc.tokenMutex.RUnlock()
		return token, nil
	}
	dmc.tokenMutex.RUnlock()

	// Need to refresh - acquire write lock
	dmc.tokenMutex.Lock()
	defer dmc.tokenMutex.Unlock()

	// Double-check after acquiring write lock (another goroutine might have refreshed)
	if dmc.cachedToken != nil && dmc.cachedToken.Valid() {
		return dmc.cachedToken, nil
	}

	log := logging.GetFromContext(ctx)

	// Retry logic with exponential backoff
	var lastErr error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * 100 * time.Millisecond
			log.Debug("retrying token refresh", "attempt", attempt+1, "backoff", backoff)
			time.Sleep(backoff)
		}

		// Use the persistent OAuth context
		token, lastErr = dmc.clientCredentials.Token(dmc.oauthCtx)
		if lastErr == nil {
			if !token.Valid() {
				lastErr = fmt.Errorf("received invalid token from %s", dmc.clientCredentials.TokenURL)
				log.Warn("received invalid token", "attempt", attempt+1)
				continue
			}
			// Success - cache the token
			dmc.cachedToken = token
			log.Debug("token refreshed successfully", "expires", token.Expiry)
			return token, nil
		}

		log.Warn("failed to refresh token", "attempt", attempt+1, "err", lastErr.Error())
	}

	err = fmt.Errorf("failed to refresh token after %d attempts: %w", maxRetries, lastErr)
	return nil, err
}

func (dmc *devManagementClient) CreateDevice(ctx context.Context, device types.Device) error {
	var err error
	ctx, span := tracer.Start(ctx, "create-device")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	url := dmc.url + "/api/v0/devices"

	requestBody, err := json.Marshal(device)
	if err != nil {
		err = fmt.Errorf("failed to marshal device: %w", err)
		return err
	}
	request := bytes.NewReader(requestBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, request)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")

	if dmc.clientCredentials != nil {
		token, err := dmc.refreshToken(ctx)
		if err != nil {
			err = fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
			return err
		}

		req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to create device: %w", err)
		return err
	}
	defer drainAndCloseResponseBody(resp)

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)

	if resp.StatusCode == http.StatusUnauthorized {
		// Invalidate cached token and try once more with fresh token
		dmc.invalidateTokenCache()
		token, retryErr := dmc.refreshToken(ctx)
		if retryErr == nil {
			// Retry the request with fresh token
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			resp2, retryErr2 := dmc.httpClient.Do(req)
			if retryErr2 == nil {
				defer drainAndCloseResponseBody(resp2)
				if resp2.StatusCode == http.StatusCreated {
					if cached, ok := dmc.knownDevEUI[device.SensorID]; ok {
						delete(dmc.cacheByInternalID, cached.internalID)
						delete(dmc.knownDevEUI, device.SensorID)
					}
					return nil
				}
				if resp2.StatusCode == http.StatusConflict {
					return ErrDeviceExist
				}
			}
		}
		err = fmt.Errorf("request failed, not authorized")
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		if resp.StatusCode == http.StatusConflict {
			return ErrDeviceExist
		}
		err = fmt.Errorf("request failed with status code %d", resp.StatusCode)
		return err
	}

	if cached, ok := dmc.knownDevEUI[device.SensorID]; ok {
		delete(dmc.cacheByInternalID, cached.internalID)
		delete(dmc.knownDevEUI, device.SensorID)
	}

	return nil
}

func (dmc *devManagementClient) GetDeviceProfile(ctx context.Context, deviceProfileID string) (*types.DeviceProfile, error) {
	var err error
	ctx, span := tracer.Start(ctx, "get-device-profile")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	params := url.Values{}
	params.Add("name", deviceProfileID)

	url := dmc.url + "/api/v0/admin/deviceprofiles?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	if dmc.clientCredentials != nil {
		token, err := dmc.refreshToken(ctx)
		if err != nil {
			err = fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve device information from devEUI: %w", err)
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)

	if resp.StatusCode == http.StatusUnauthorized {
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
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
		Data types.DeviceProfile `json:"data"`
	}{}

	err = json.Unmarshal(respBody, &responseData)
	if err != nil {
		err = fmt.Errorf("failed to unmarshal response body: %w", err)
		return nil, err
	}

	deviceprofile := responseData.Data
	return &deviceprofile, nil
}

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

var ErrNotFound error = errors.New("not found")

var errInternal error = errors.New("internal error")
var errRetry error = errors.New("retry")

func (dmc *devManagementClient) FindDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error) {

	resultchan := make(chan Device)
	errchan := make(chan error)

	dmc.queue <- func() {
		device, ok := dmc.knownDevEUI[devEUI]

		if ok {
			switch device.state {
			case Ready:
				go func() {
					deviceByID, err := dmc.FindDeviceFromInternalID(ctx, device.internalID)
					if err != nil {
						errchan <- err
					} else {
						resultchan <- deviceByID
					}
				}()
			case Error:
				if time.Since(device.when) > dmc.errorCacheTTL {
					dmc.knownDevEUI[devEUI] = devEUIState{state: Refreshing, when: time.Now()}
					go func() {
						dmc.updateDeviceCacheFromDevEUI(ctx, devEUI)
					}()
					errchan <- errRetry
					return
				}
				errchan <- device.err
			case Refreshing:
				errchan <- errRetry
			default:
				errchan <- errInternal
			}

			return
		}

		dmc.knownDevEUI[devEUI] = devEUIState{state: Refreshing, when: time.Now()}
		go func() {
			dmc.updateDeviceCacheFromDevEUI(ctx, devEUI)
		}()
		errchan <- errRetry
	}

	select {
	case d := <-resultchan:
		return d, nil
	case e := <-errchan:
		if errors.Is(e, errRetry) {
			time.Sleep(10 * time.Millisecond)
			return dmc.FindDeviceFromDevEUI(ctx, devEUI)
		}
		return nil, e
	}
}

func (dmc *devManagementClient) updateDeviceCacheFromDevEUI(ctx context.Context, devEUI string) error {
	device, err := dmc.findDeviceFromDevEUI(ctx, devEUI)

	dmc.queue <- func() {
		if err != nil {
			log := logging.GetFromContext(ctx)

			if errors.Is(err, ErrNotFound) {
				log.Info("device not found", "devEUI", devEUI)
			} else {
				log.Error("failed to update device cache", "err", err.Error())
			}

			dmc.knownDevEUI[devEUI] = devEUIState{state: Error, err: err, when: time.Now()}
		} else {
			dmc.knownDevEUI[devEUI] = devEUIState{state: Ready, internalID: device.ID(), when: time.Now()}
			dmc.cacheByInternalID[device.ID()] = lookupResult{state: Ready, device: device, when: time.Now()}
		}
	}

	return err
}

func (dmc *devManagementClient) findDeviceFromDevEUI(ctx context.Context, devEUI string) (Device, error) {
	var err error
	ctx, span := tracer.Start(ctx, "find-device-from-deveui")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx)
	log.Info("looking up internal id and types", "devEUI", devEUI)

	url := dmc.url + "/api/v0/devices?devEUI=" + devEUI

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	if dmc.clientCredentials != nil {
		token, err := dmc.refreshToken(ctx)
		if err != nil {
			err = fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve device information from devEUI: %w", err)
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)

	if resp.StatusCode == http.StatusUnauthorized {
		// Invalidate cached token and try once more with fresh token
		dmc.invalidateTokenCache()
		token, retryErr := dmc.refreshToken(ctx)
		if retryErr == nil {
			// Retry the request with fresh token
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			resp2, retryErr2 := dmc.httpClient.Do(req)
			if retryErr2 == nil {
				defer drainAndCloseResponseBody(resp2)
				if resp2.StatusCode == http.StatusOK {
					respBody, err := io.ReadAll(resp2.Body)
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
				if resp2.StatusCode == http.StatusNotFound {
					return nil, ErrNotFound
				}
			}
		}
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
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

	resultchan := make(chan Device)
	errchan := make(chan error)

	dmc.queue <- func() {
		r, ok := dmc.cacheByInternalID[deviceID]

		if ok {
			switch r.state {
			case Ready:
				resultchan <- r.device
			case Error:
				if time.Since(r.when) > dmc.errorCacheTTL {
					dmc.cacheByInternalID[deviceID] = lookupResult{state: Refreshing, when: time.Now()}
					go func() {
						dmc.updateDeviceCacheFromInternalID(ctx, deviceID)
					}()
					errchan <- errRetry
					return
				}
				errchan <- r.err
			case Refreshing:
				errchan <- errRetry
			default:
				errchan <- errInternal
			}

			return
		}

		// if not in cache, send request to device management
		r = lookupResult{state: Refreshing, when: time.Now()}
		dmc.cacheByInternalID[deviceID] = r

		go func() {
			dmc.updateDeviceCacheFromInternalID(ctx, deviceID)
		}()

		errchan <- errRetry
	}

	select {
	case d := <-resultchan:
		return d, nil
	case e := <-errchan:
		if errors.Is(e, errRetry) {
			time.Sleep(10 * time.Millisecond)
			return dmc.FindDeviceFromInternalID(ctx, deviceID)
		}
		return nil, e
	}
}

func (dmc *devManagementClient) updateDeviceCacheFromInternalID(ctx context.Context, deviceID string) error {
	device, err := dmc.findDeviceFromInternalID(ctx, deviceID)

	dmc.queue <- func() {
		if err != nil {
			log := logging.GetFromContext(ctx)
			log.Error("failed to update device cache", "err", err.Error())

			dmc.cacheByInternalID[deviceID] = lookupResult{state: Error, err: err, when: time.Now()}
		} else {
			dmc.cacheByInternalID[deviceID] = lookupResult{state: Ready, device: device, when: time.Now()}
		}
	}

	return err
}

func (dmc *devManagementClient) findDeviceFromInternalID(ctx context.Context, deviceID string) (Device, error) {
	var err error
	ctx, span := tracer.Start(ctx, "find-device-from-id")
	defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

	log := logging.GetFromContext(ctx)
	log.Info("looking up properties for device", "device_id", deviceID)

	url := dmc.url + "/api/v0/devices/" + deviceID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		err = fmt.Errorf("failed to create http request: %w", err)
		return nil, err
	}

	if dmc.clientCredentials != nil {
		token, err := dmc.refreshToken(ctx)
		if err != nil {
			err = fmt.Errorf("failed to get client credentials from %s: %w", dmc.clientCredentials.TokenURL, err)
			return nil, err
		}

		req.Header.Add("Authorization", "Bearer "+token.AccessToken)
	}

	resp, err := dmc.httpClient.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to retrieve information for device: %w", err)
		return nil, err
	}
	defer drainAndCloseResponseBody(resp)

	dmc.dumpRequestResponseIfNon200AndDebugEnabled(ctx, req, resp)

	if resp.StatusCode == http.StatusUnauthorized {
		// Invalidate cached token and try once more with fresh token
		dmc.invalidateTokenCache()
		token, retryErr := dmc.refreshToken(ctx)
		if retryErr == nil {
			// Retry the request with fresh token
			req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			resp2, retryErr2 := dmc.httpClient.Do(req)
			if retryErr2 == nil {
				defer drainAndCloseResponseBody(resp2)
				if resp2.StatusCode == http.StatusOK {
					respBody, err := io.ReadAll(resp2.Body)
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
				if resp2.StatusCode == http.StatusNotFound {
					return nil, ErrNotFound
				}
			}
		}
		err = fmt.Errorf("request failed, not authorized")
		return nil, err
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
	return d.impl.Tenant
}

func (d *deviceWrapper) Source() string {
	return d.impl.Source
}
