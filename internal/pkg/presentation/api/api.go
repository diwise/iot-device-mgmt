package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api/auth"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(log zerolog.Logger, router *chi.Mux, policies io.Reader, app application.DeviceManagement) *chi.Mux {

	router.Get("/health", NewHealthHandler(log, app))

	router.Route("/api/v0", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			// Handle valid / invalid tokens.
			authenticator, err := auth.NewAuthenticator(context.Background(), log, policies)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create api authenticator")
			}
			r.Use(authenticator)

			r.Route("/devices", func(r chi.Router) {
				r.Get("/", queryDevicesHandler(log, app))
				r.Post("/", createDeviceHandler(log, app))
				r.Get("/{id}", retrieveDeviceHandler(log, app))
				r.Patch("/{id}", patchDeviceHandler(log, app))
			})

			r.Get("/events", sseHandler(log, app))
			r.Get("/environments", listEnvironments(log, app))
		})
	})

	return router
}

func NewHealthHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func listEnvironments(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "list-environments")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		env, err := app.ListEnvironments(ctx)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to list environments")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bytes, err := json.Marshal(&env)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal environments to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

func createDeviceHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "create-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to read body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var d application.Device
		err = json.Unmarshal(body, &d)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to unmarshal body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = app.CreateDevice(ctx, d)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to create device")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
	}
}

func patchDeviceHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "patch-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "id")

		b, err := io.ReadAll(r.Body)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to read body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var fields map[string]interface{}
		err = json.Unmarshal(b, &fields)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to unmarshal body into map")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		_, err = app.UpdateDevice(ctx, deviceID, fields)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to update device")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

func queryDevicesHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "query-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceArray := []application.Device{}

		devEUI := r.URL.Query().Get("devEUI")
		if devEUI == "" {
			devices, err := app.ListAllDevices(ctx, allowedTenants)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to fetch all devices")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			deviceArray = append(deviceArray, devices...)
			requestLogger.Info().Msgf("returning information about %d devices", len(devices))
		} else {
			device, err := app.GetDeviceFromEUI(ctx, devEUI)
			if err != nil {
				requestLogger.Error().Err(err).Msg("device not found")
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if notInAllowedTenants(device.Tenant, allowedTenants) {
				err = fmt.Errorf("client not allowed to access tenant %s", device.Tenant)
				requestLogger.Error().Err(err).Msg("not authorized")
				w.WriteHeader(http.StatusNotFound)
				return
			}

			deviceArray = append(deviceArray, device)
			requestLogger.Info().Msgf("returning information about device %s", device.DeviceId)
		}

		bytes, err := json.Marshal(&deviceArray)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal device to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

func retrieveDeviceHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "id")
		device, err := app.GetDevice(ctx, deviceID)
		if err != nil {
			requestLogger.Error().Err(err).Msg("device not found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if notInAllowedTenants(device.Tenant, allowedTenants) {
			err = fmt.Errorf("client not allowed to access tenant %s", device.Tenant)
			requestLogger.Error().Err(err).Msg("not authorized")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		bytes, err := json.Marshal(device)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal device to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		requestLogger.Info().Msgf("returning information about device %s (%s)", device.DeviceId, deviceID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

func sseHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())
		client := application.NewClient(r.RemoteAddr, allowedTenants)

		app.RegisterClient(r.Context(), client)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		timeout := time.After(1 * time.Second)
		select {
		case ev := <-client.Evt():
			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			enc.Encode(ev)
			fmt.Fprintf(w, "data: %v\n\n", buf.String())
			fmt.Printf("data: %v\n", buf.String())
		case <-timeout:
			fmt.Fprintf(w, ": nothing to sent\n\n")
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.Encode(&application.Event{Type: "clientConnected"})
		fmt.Fprintf(w, "data: %v\n\n", buf.String())
	}
}

func notInAllowedTenants(tenant string, allowedTenants []string) bool {
	for _, t := range allowedTenants {
		if t == tenant {
			return false
		}
	}

	return true
}
