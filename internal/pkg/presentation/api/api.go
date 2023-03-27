package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/service"
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/models"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api/auth"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(log zerolog.Logger, router *chi.Mux, policies io.Reader, service service.DeviceManagement) *chi.Mux {

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	router.Route("/api/v0", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			// Handle valid / invalid tokens.
			authenticator, err := auth.NewAuthenticator(context.Background(), log, policies)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create api authenticator")
			}
			r.Use(authenticator)

			/*

				GET
				/api/v0/devices - get all
				/api/v0/devices/stats - total, online, warning ...
				/api/v0/devices/:deviceID - single
				/api/v0/devices/:deviceID/alarms - all alarms

				POST
				/api/v0/devices - create new device

				PATCH
				/api/v0/devices/:deviceID - update device

			*/

			r.Route("/devices", func(r chi.Router) {
				r.Get("/", queryDevicesHandler(log, service))
				r.Get("/{deviceID}", getDeviceDetails(log, service))
				r.Get("/{deviceID}/alarms", getAlarmsHandler(log, service))

				r.Get("/alarms", getAlarmsHandler(log, service))

				r.Post("/", createDeviceHandler(log, service))
				r.Patch("/{deviceID}", patchDeviceHandler(log, service))
			})
		})

	})

	return router
}

func createDeviceHandler(log zerolog.Logger, service service.DeviceManagement) http.HandlerFunc {
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

		var d types.Device
		err = json.Unmarshal(body, &d)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to unmarshal body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = service.CreateDevice(ctx, d)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to create device")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
	}
}

func patchDeviceHandler(log zerolog.Logger, service service.DeviceManagement) http.HandlerFunc {
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

		err = service.UpdateDevice(ctx, deviceID, fields)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to update device")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

func queryDevicesHandler(log zerolog.Logger, service service.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "query-all-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		var devices []models.Device

		sensorID := r.URL.Query().Get("devEUI")
		if sensorID != "" {
			device, err := service.GetDeviceBySensorID(ctx, sensorID, allowedTenants...)
			if errors.Is(err, db.ErrDeviceNotFound) {
				requestLogger.Error().Err(err).Msgf("%s not found", sensorID)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if err != nil {
				requestLogger.Error().Err(err).Msg("could not fetch data")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			devices = append(devices, device)
		} else {
			devices, err = service.GetDevices(ctx, allowedTenants...)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to fetch all devices")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		// TODO: map to types.Devices?

		b, err := json.Marshal(devices)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to fetch all devices")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}
}

func getAlarmsHandler(log zerolog.Logger, service service.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-alarms")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "deviceID")

		if deviceID == "" {
			onlyActive := r.URL.Query().Get("active") == "true"
			alarms, err := service.GetAlarms(ctx, onlyActive)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to fetch alarms")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			b, err := json.Marshal(alarms)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to marshal alarms")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(b)
			return
		}

		device, err := service.GetDeviceByDeviceID(ctx, deviceID, allowedTenants...)
		if errors.Is(err, db.ErrDeviceNotFound) {
			requestLogger.Debug().Msgf("%s not found", deviceID)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			requestLogger.Error().Err(err).Msg("could not fetch data")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		b, err := json.Marshal(device.Alarms)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to fetch alarms")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}
}

func getDeviceDetails(log zerolog.Logger, service service.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "deviceID")

		device, err := service.GetDeviceByDeviceID(ctx, deviceID, allowedTenants...)
		if errors.Is(err, db.ErrDeviceNotFound) {
			requestLogger.Debug().Msgf("%s not found", deviceID)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			requestLogger.Error().Err(err).Msg("could not fetch data")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		//TODO: map to type.Device?

		bytes, err := json.Marshal(device)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal device to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		requestLogger.Info().Msgf("returning information about device %s (%s)", device.DeviceID, deviceID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}
