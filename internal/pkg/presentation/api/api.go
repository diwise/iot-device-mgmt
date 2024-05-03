package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"log/slog"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api/auth"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(ctx context.Context, router *chi.Mux, policies io.Reader, svc devicemanagement.DeviceManagement, alarmSvc alarms.AlarmService) (*chi.Mux, error) {
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	log := logging.GetFromContext(ctx)

	// Handle valid / invalid tokens.
	authenticator, err := auth.NewAuthenticator(ctx, log, policies)
	if err != nil {
		return nil, fmt.Errorf("failed to create api authenticator: %w", err)
	}

	router.Route("/api/v0", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authenticator)

			r.Route("/devices", func(r chi.Router) {
				r.Get("/", queryDevicesHandler(log, svc))
				r.Get("/{deviceID}", getDeviceDetails(log, svc))
				r.Post("/", createDeviceHandler(log, svc))
				r.Patch("/{deviceID}", patchDeviceHandler(log, svc))
			})

			//r.Get("/alarms", getAlarmsHandler(log, alarmSvc))
			//r.Patch("/alarms/{alarmID}", patchAlarmsHandler(log, alarmSvc))
		})

	})

	return router, nil
}

type meta struct {
	TotalRecords uint64  `json:"totalRecords"`
	Offset       *uint64 `json:"offset,omitempty"`
	Limit        *uint64 `json:"limit,omitempty"`
	Count        uint64  `json:"count"`
}

type links struct {
	Self  *string `json:"self,omitempty"`
	First *string `json:"first,omitempty"`
	Prev  *string `json:"prev,omitempty"`
	Next  *string `json:"next,omitempty"`
	Last  *string `json:"last,omitempty"`
}

func createLinks(baseUrl string, m meta) *links {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil
	}
	query := u.Query()
	newUrl := func(offset uint64) *string {
		query.Set("offset", strconv.Itoa(int(offset)))
		u.RawQuery = query.Encode()
		u_ := u.String()
		return &u_
	}
	f := uint64(0)
	l := ((m.TotalRecords - 1) / *m.Limit) * *m.Limit
	n := *m.Offset + *m.Limit
	p := *m.Offset - *m.Limit
	links := &links{
		Self:  newUrl(*m.Offset),
		First: newUrl(f),
		Last:  newUrl(l),
	}
	if n < m.TotalRecords {
		links.Next = newUrl(n)
	}
	if p >= 0 {
		links.Prev = newUrl(p)
	}
	return links
}

type CollectionResponse struct {
	Meta  *meta  `json:"meta,omitempty"`
	Data  any    `json:"data"`
	Links *links `json:"links,omitempty"`
}

func (r CollectionResponse) Body() []byte {
	b, _ := json.Marshal(r)
	return b
}

func getOffsetAndLimit(r *http.Request) (int, int) {
	offset := r.URL.Query().Get("offset")
	limit := r.URL.Query().Get("limit")

	conv := func(s string, defaultValue int) int {
		i, err := strconv.Atoi(s)
		if err != nil {
			return defaultValue
		}
		return i
	}

	return conv(offset, 0), conv(limit, 10)
}

func queryDevicesHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "query-all-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		sensorID := r.URL.Query().Get("devEUI")

		if sensorID != "" {
			device, err := svc.GetDeviceBySensorID(ctx, sensorID, allowedTenants)
			if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
				requestLogger.Debug("device not found", "sensor_id", sensorID)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if err != nil {
				requestLogger.Error("could not fetch data", "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			response := CollectionResponse{
				Data: device,
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response.Body())
			return
		} else {
			offset, limit := getOffsetAndLimit(r)
			collection, err := svc.GetDevices(ctx, offset, limit, allowedTenants)
			if err != nil {
				requestLogger.Error("unable to fetch devices", "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			meta := &meta{
				TotalRecords: collection.TotalCount,
				Offset:       &collection.Offset,
				Limit:        &collection.Limit,
				Count:        collection.Count,
			}

			response := CollectionResponse{
				Meta:  meta,
				Data:  collection.Data,
				Links: createLinks(r.URL.Path, *meta),
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response.Body())
			return
		}
	}
}

func getDeviceDetails(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "deviceID")
		if deviceID != "" {
			requestLogger = requestLogger.With(slog.String("device_id", deviceID))
		}

		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		device, err := svc.GetDeviceByDeviceID(ctx, deviceID, allowedTenants)
		if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
			requestLogger.Debug("device not found")
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err != nil {
			requestLogger.Error("could not fetch data", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := CollectionResponse{
			Data: device,
		}

		requestLogger.Info("returning information about device")

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Body())
	}
}

func createDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		ctx, span := tracer.Start(r.Context(), "create-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		if isMultipartFormData(r) {
			file, _, err := r.FormFile("fileupload")
			if err != nil {
				requestLogger.Error("unable to read file", "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = svc.Import(ctx, file, allowedTenants)
			if err != nil {
				requestLogger.Error("failed to import data", "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			return
		} else if isApplicationJson(r) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				requestLogger.Error("unable to read body", "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			var d types.Device
			err = json.Unmarshal(body, &d)
			if err != nil {
				requestLogger.Error("unable to unmarshal body", "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = svc.CreateDevice(ctx, d)
			if err != nil {
				requestLogger.Error("unable to create device", "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			return
		}

		requestLogger.Error("Unsupported MediaType")
		w.WriteHeader(http.StatusUnsupportedMediaType)
	}
}

func patchDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		ctx, span := tracer.Start(r.Context(), "patch-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "deviceID")
		if deviceID == "" {
			requestLogger.Error("no deviceID in request", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		requestLogger = requestLogger.With(slog.String("device_id", deviceID))

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		b, err := io.ReadAll(r.Body)
		if err != nil {
			requestLogger.Error("unable to read body", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var fields map[string]any
		err = json.Unmarshal(b, &fields)
		if err != nil {
			requestLogger.Error("unable to unmarshal body into map", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.UpdateDevice(ctx, deviceID, fields, allowedTenants)
		if err != nil {
			requestLogger.Error("unable to update device", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}
/*
func getAlarmsHandler(log *slog.Logger, svc alarms.AlarmService) http.HandlerFunc {
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
			requestLogger.Error("unable to fetch alarms", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		b, err := json.Marshal(alarms)
		if err != nil {
			requestLogger.Error("unable to marshal alarms", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)
	}
}

func patchAlarmsHandler(log *slog.Logger, svc alarms.AlarmService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

		//allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "delete-alarms")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		id := chi.URLParam(r, "alarmID")
		if id != "" {
			requestLogger = requestLogger.With(slog.String("alarm_id", id))
		}

		alarmID, err := strconv.Atoi(id)
		if err != nil {
			requestLogger.Error("id is invalid", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.CloseAlarm(ctx, alarmID)
		if err != nil {
			requestLogger.Error("unable to close alarm", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
	}
}
*/
func isMultipartFormData(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "multipart/form-data")
}

func isApplicationJson(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}
