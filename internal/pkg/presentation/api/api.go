package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	aDb "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	dmDb "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api/auth"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(ctx context.Context, router *chi.Mux, policies io.Reader, svc devicemanagement.DeviceManagement, alarmSvc alarms.AlarmService) *chi.Mux {

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	log := logging.GetFromContext(ctx)

	router.Route("/api/v0", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			// Handle valid / invalid tokens.
			authenticator, err := auth.NewAuthenticator(ctx, log, policies)
			if err != nil {
				log.Fatal().Err(err).Msg("failed to create api authenticator")
			}
			r.Use(authenticator)

			r.Route("/devices", func(r chi.Router) {
				r.Get("/", queryDevicesHandler(log, svc))
				r.Get("/{deviceID}", getDeviceDetails(log, svc))
				r.Post("/", createDeviceHandler(log, svc))
				r.Patch("/{deviceID}", patchDeviceHandler(log, svc))
			})

			r.Get("/alarms", getAlarmsHandler(log, alarmSvc))
			r.Patch("/alarms/{alarmID}", patchAlarmsHandler(log, alarmSvc))
		})

	})

	return router
}

func createDeviceHandler(log zerolog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		ctx, span := tracer.Start(r.Context(), "create-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		if isMultipartFormData(r) {
			file, _, err := r.FormFile("fileupload")
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to read file")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = svc.Import(ctx, file)
			if err != nil {
				requestLogger.Error().Err(err).Msg("failed to import data")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			return
		} else if isAapplicationJson(r) {
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

			err = svc.CreateDevice(ctx, d)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to create device")
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			return
		}

		requestLogger.Error().Err(err).Msg("Unsupported MediaType")
		w.WriteHeader(http.StatusUnsupportedMediaType)
	}
}

func isMultipartFormData(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "multipart/form-data")
}

func isAapplicationJson(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

func queryDevicesHandler(log zerolog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "query-all-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		var devices []types.Device

		sensorID := r.URL.Query().Get("devEUI") // TODO: change to sensorID?
		if sensorID != "" {
			device, err := svc.GetDeviceBySensorID(ctx, sensorID, allowedTenants...)
			if errors.Is(err, dmDb.ErrDeviceNotFound) {
				requestLogger.Debug().Msgf("%s not found", sensorID)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if err != nil {
				requestLogger.Error().Err(err).Msg("could not fetch data")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			d, err := devicemanagement.MapTo[types.Device](device)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable map device")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			devices = append(devices, d)
		} else {
			fromDb, err := svc.GetDevices(ctx, allowedTenants...)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to fetch all devices")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			for _, device := range fromDb {
				d, err := devicemanagement.MapTo[types.Device](device)
				if err != nil {
					requestLogger.Error().Err(err).Msg("unable map device")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				devices = append(devices, d)
			}
		}

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

func getDeviceDetails(log zerolog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "deviceID")

		device, err := svc.GetDeviceByDeviceID(ctx, deviceID, allowedTenants...)
		if errors.Is(err, dmDb.ErrDeviceNotFound) {
			requestLogger.Debug().Msgf("%s not found", deviceID)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			requestLogger.Error().Err(err).Msg("could not fetch data")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		d, err := devicemanagement.MapTo[types.Device](device)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable map device")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bytes, err := json.Marshal(d)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal device to json")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		requestLogger.Info().Msgf("returning information about device id: %s, url param: %s", device.DeviceID, deviceID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(bytes)
	}
}

func patchDeviceHandler(log zerolog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		ctx, span := tracer.Start(r.Context(), "patch-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "deviceID")

		b, err := io.ReadAll(r.Body)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to read body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var fields map[string]any
		err = json.Unmarshal(b, &fields)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to unmarshal body into map")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.UpdateDevice(ctx, deviceID, fields)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to update device")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

func getAlarmsHandler(log zerolog.Logger, svc alarms.AlarmService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-alarms")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		var alarms []aDb.Alarm

		refID := r.URL.Query().Get("refID")

		if len(refID) > 0 {
			alarms, err = svc.GetAlarmsByRefID(ctx, refID, allowedTenants...)
		} else {
			alarms, err = svc.GetAlarms(ctx, allowedTenants...)
		}
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
	}
}

func patchAlarmsHandler(log zerolog.Logger, svc alarms.AlarmService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		//allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "delete-alarms")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		id := chi.URLParam(r, "alarmID")
		alarmID, err := strconv.Atoi(id)
		if err != nil {
			requestLogger.Error().Err(err).Msg("id is invalid")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.CloseAlarm(ctx, alarmID)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to close alarm")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}
}
