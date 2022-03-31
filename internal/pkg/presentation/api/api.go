package api

import (
	"encoding/json"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(logging zerolog.Logger, router *chi.Mux, app application.DeviceManagement) *chi.Mux {

	router.Get("/health", NewHealthHandler(logging, app))
	router.Get("/api/v0/devices/{id}", NewDeviceHandler(logging, app))

	return router
}

func NewHealthHandler(logging zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
}

func NewDeviceHandler(logging zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
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

		traceID := span.SpanContext().TraceID()
		if traceID.IsValid() {
			logging = logging.With().Str("traceID", traceID.String()).Logger()
		}

		deviceID := chi.URLParam(r, "id")
		device, err := app.GetDevice(ctx, deviceID)
		if err != nil {
			logging.Error().Err(err).Msg("device not found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		bytes, err := json.Marshal(device)
		if err != nil {
			logging.Error().Err(err).Msg("unable to marshal device to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		logging.Info().Msgf("returning information about device %s (%s)", device.ID(), deviceID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}
