package api

import (
	"encoding/json"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/logging"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(log zerolog.Logger, router *chi.Mux, app application.DeviceManagement) *chi.Mux {

	router.Get("/health", NewHealthHandler(log, app))
	router.Get("/api/v0/devices", NewQueryDevicesHandler(log, app))
	router.Post("/api/v0/devices", NewRegisterNewDeviceHandler(log, app))
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
		ctx := r.Context()

		ctx, span := tracer.Start(ctx, "query-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

		requestLogger := log

		traceID := span.SpanContext().TraceID()
		if traceID.IsValid() {
			requestLogger = requestLogger.With().Str("traceID", traceID.String()).Logger()
		}

		ctx = logging.NewContextWithLogger(ctx, requestLogger)

		devEUI := r.URL.Query().Get("devEUI")
		if devEUI == "" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("query only supported from devEUI at this point"))
			return
		}

		device, err := app.GetDeviceFromEUI(ctx, devEUI)
		if err != nil {
			requestLogger.Error().Err(err).Msg("device not found")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		deviceArray := []database.Device{device}
		bytes, err := json.Marshal(&deviceArray)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal device to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		requestLogger.Info().Msgf("returning information about device %s", device.ID())

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

func NewRetrieveDeviceHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
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
func NewRegisterNewDeviceHandler(log zerolog.Logger, app application.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		ctx, span := tracer.Start(ctx, "register-new-device")
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

		// TODO:
		// unmarshal request body
		// create new device
		// register new device in repository
		// return new device

		d := struct {
			DevEUI      string   `json:"devEUI"`
			Latitude    float64  `json:"latitude"`
			Longitude   float64  `json:"longitude"`
			Environment string   `json:"environment"`
			Types       []string `json:"types"`
			SensorType  string   `json:"sensorType"`
		}{}

		err = json.NewDecoder(r.Body).Decode(&d)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
	}
}
