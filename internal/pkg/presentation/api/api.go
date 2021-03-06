package api

import (
	"encoding/json"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(log zerolog.Logger, router *chi.Mux, app application.DeviceManagement) *chi.Mux {
	router.Get("/health", NewHealthHandler(log, app))
	router.Get("/api/v0/devices", NewQueryDevicesHandler(log, app))
	router.Get("/api/v0/devices/{id}", NewRetrieveDeviceHandler(log, app))

	return router
}

func NewHealthHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func NewQueryDevicesHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "query-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

		requestLogger := log

		traceID := span.SpanContext().TraceID()
		if traceID.IsValid() {
			requestLogger = requestLogger.With().Str("traceID", traceID.String()).Logger()
		}

		ctx = logging.NewContextWithLogger(ctx, requestLogger)

		deviceArray := []database.Device{}

		devEUI := r.URL.Query().Get("devEUI")
		if devEUI == "" {
			devices, err := app.ListAllDevices(ctx)
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
			deviceArray = append(deviceArray, device)
			requestLogger.Info().Msgf("returning information about device %s", device.ID())
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

func NewRetrieveDeviceHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

		requestLogger := log

		traceID := span.SpanContext().TraceID()
		if traceID.IsValid() {
			requestLogger = requestLogger.With().Str("traceID", traceID.String()).Logger()
		}

		ctx = logging.NewContextWithLogger(ctx, requestLogger)

		deviceID := chi.URLParam(r, "id")
		device, err := app.GetDevice(ctx, deviceID)
		if err != nil {
			requestLogger.Error().Err(err).Msg("device not found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		bytes, err := json.Marshal(device)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal device to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		requestLogger.Info().Msgf("returning information about device %s (%s)", device.ID(), deviceID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}
