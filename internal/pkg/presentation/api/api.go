package api

import (
	"encoding/json"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/logging"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(log zerolog.Logger, router *chi.Mux, app application.DeviceManagement) *chi.Mux {

	router.Get("/health", NewHealthHandler(log, app))
	router.Get("/api/v0/devices/{id}", NewDeviceHandler(log, app))

	return router
}

func NewHealthHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func NewDeviceHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		ctx, span := tracer.Start(ctx, "get-device")
		defer func() {
			if err != nil {
				span.RecordError(err)
			}
			span.End()
		}()

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
