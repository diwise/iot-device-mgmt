package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
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
				r.Get("/{deviceID}/alarms", getAlarmsHandler(log, alarmSvc))
				r.Post("/", createDeviceHandler(log, svc))
				r.Put("/{deviceID}", updateDeviceHandler(log, svc))
				r.Patch("/{deviceID}", patchDeviceHandler(log, svc))
			})

			r.Route("/alarms", func(r chi.Router) {
				r.Get("/", getAlarmsHandler(log, alarmSvc))
				r.Get("/{alarmID}", getAlarmDetailsHandler(log, alarmSvc))
				r.Patch("/{alarmID}/close", closeAlarmHandler(log, alarmSvc))
			})

			r.Route("/admin", func(r chi.Router) {
				r.Get("/deviceprofiles", queryDeviceProfilesHandler(log, svc))
				r.Get("/deviceprofiles/{deviceprofileid}", queryDeviceProfilesHandler(log, svc))
				r.Get("/lwm2mtypes", queryLwm2mTypesHandler(log, svc))
				r.Get("/tenants", queryTenantsHandler())
			})
		})
	})

	return router, nil
}

func createLinks(u *url.URL, m *meta) *links {
	if m == nil || m.TotalRecords == 0 {
		return nil
	}

	query := u.Query()

	newUrl := func(offset uint64) *string {
		query.Set("offset", strconv.Itoa(int(offset)))
		u.RawQuery = query.Encode()
		u_ := u.String()
		return &u_
	}

	first := uint64(0)
	last := ((m.TotalRecords - 1) / *m.Limit) * *m.Limit
	next := *m.Offset + *m.Limit
	prev := int64(*m.Offset) - int64(*m.Limit)

	links := &links{
		Self:  newUrl(*m.Offset),
		First: newUrl(first),
		Last:  newUrl(last),
	}

	if next < m.TotalRecords {
		links.Next = newUrl(next)
	}

	if prev >= 0 {
		links.Prev = newUrl(uint64(prev))
	}

	return links
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

func queryTenantsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		_, span := tracer.Start(r.Context(), "query-tenants")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

		response := ApiResponse{
			Data: allowedTenants,
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func queryDevicesHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "query-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		sensorID := r.URL.Query().Get("devEUI")

		if sensorID != "" {
			ctx = logging.NewContextWithLogger(ctx, requestLogger, slog.String("sensor_id", sensorID))

			device, err := svc.GetBySensorID(ctx, sensorID, allowedTenants)
			if err != nil {
				if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				requestLogger.Error("could not fetch data", "sensor_id", sensorID, "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			response := ApiResponse{
				Data: device,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response.Byte())

			return
		} else {
			collection, err := svc.Query(ctx, r.URL.Query(), allowedTenants)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			meta := &meta{
				TotalRecords: collection.TotalCount,
				Offset:       &collection.Offset,
				Limit:        &collection.Limit,
				Count:        collection.Count,
			}

			links := createLinks(r.URL, meta)

			if wantsGeoJSON(r) {
				response, err := NewFeatureCollectionWithDevices(collection.Data)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				response.Meta = meta
				response.Links = links

				b, err := json.Marshal(response)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				w.Header().Set("Content-Type", "application/geo+json")
				w.WriteHeader(http.StatusOK)
				w.Write(b)

				return
			}

			if wantsTextCSV(r) {
				w.Header().Set("Content-Type", "text/csv")

				err := writeCsvWithDevices(w, collection.Data)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusOK)
				return
			}

			response := ApiResponse{
				Meta:  meta,
				Data:  collection.Data,
				Links: links,
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response.Byte())
			return
		}
	}
}

func getDeviceDetails(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := chi.URLParam(r, "deviceID")
		ctx = logging.NewContextWithLogger(ctx, requestLogger, slog.String("device_id", deviceID))

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		device, err := svc.GetByDeviceID(ctx, deviceID, allowedTenants)
		if err != nil {
			if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			requestLogger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{
			Data: device,
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func createDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

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

			err = svc.Seed(ctx, file, allowedTenants)
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
				requestLogger.Error("unable to unmarshal body", "body", string(body), "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if !slices.Contains(allowedTenants, d.Tenant) {
				requestLogger.Error("not allowed to create device with current tenant", "device_id", d.DeviceID, "tenant", d.Tenant)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			err = svc.Create(ctx, d)
			if err != nil {
				if errors.Is(err, devicemanagement.ErrDeviceAlreadyExist) {
					w.WriteHeader(http.StatusConflict)
					return
				}

				requestLogger.Error("unable to create device", "device_id", d.DeviceID, "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
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

func updateDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "update-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		if !isApplicationJson(r) {
			requestLogger.Error("Unsupported MediaType")
			w.WriteHeader(http.StatusUnsupportedMediaType)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			requestLogger.Error("unable to read body", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var d types.Device
		err = json.Unmarshal(body, &d)
		if err != nil {
			requestLogger.Error("unable to unmarshal body", "body", string(body), "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !slices.Contains(allowedTenants, d.Tenant) {
			requestLogger.Error("not allowed to update device with current tenant", "device_id", d.DeviceID, "tenant", d.Tenant)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		err = svc.Update(ctx, d)
		if err != nil {
			requestLogger.Error("unable to create device", "device_id", d.DeviceID, "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

func patchDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "patch-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, requestLogger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())
		deviceID := chi.URLParam(r, "deviceID")

		ctx = logging.NewContextWithLogger(ctx, requestLogger, slog.String("device_id", deviceID))

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

		err = svc.Merge(ctx, deviceID, fields, allowedTenants)
		if err != nil {
			requestLogger.Error("unable to update device", "device_id", deviceID, "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

func getAlarmsHandler(log *slog.Logger, svc alarms.AlarmService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "get-alarms")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, log = o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())
		var collection types.Collection[types.Alarm]
		offset, limit := getOffsetAndLimit(r)

		deviceID := chi.URLParam(r, "deviceID")

		if deviceID != "" {
			ctx = logging.NewContextWithLogger(ctx, log, slog.String("device_id", deviceID))
			collection, err = svc.GetByRefID(ctx, deviceID, offset, limit, allowedTenants)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		} else {
			asInfo := r.URL.Query().Get("info") == "true"
			if asInfo {
				info, err := svc.Info(ctx, offset, limit, allowedTenants)
				if err != nil {
					w.WriteHeader(http.StatusNotFound)
					return
				}
				meta := &meta{
					TotalRecords: info.TotalCount,
					Offset:       &info.Offset,
					Limit:        &info.Limit,
					Count:        info.Count,
				}

				response := ApiResponse{
					Meta:  meta,
					Data:  info.Data,
					Links: createLinks(r.URL, meta),
				}
				w.Header().Add("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write(response.Byte())

				return
			}

			collection, err = svc.Get(ctx, offset, limit, allowedTenants)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		meta := &meta{
			TotalRecords: collection.TotalCount,
			Offset:       &collection.Offset,
			Limit:        &collection.Limit,
			Count:        collection.Count,
		}

		response := ApiResponse{
			Meta:  meta,
			Data:  collection.Data,
			Links: createLinks(r.URL, meta),
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func getAlarmDetailsHandler(log *slog.Logger, svc alarms.AlarmService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "get-alarm")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, _ = o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		alarmID := chi.URLParam(r, "alarmID")
		if alarmID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		alarm, err := svc.GetByID(ctx, alarmID, allowedTenants)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		response := ApiResponse{
			Data: alarm,
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func closeAlarmHandler(log *slog.Logger, svc alarms.AlarmService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "close-alarm")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, _ = o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		alarmID := chi.URLParam(r, "alarmID")
		if alarmID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		alarm, err := svc.GetByID(ctx, alarmID, allowedTenants)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.Close(ctx, alarm.ID, allowedTenants)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

func queryDeviceProfilesHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "query-deviceprofiles")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, _ = o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		name := r.URL.Query().Get("name")
		deviceprofileId := chi.URLParam(r, "deviceprofileid")

		if name == "" && deviceprofileId != "" {
			name = deviceprofileId
		}

		var profiles types.Collection[types.DeviceProfile]

		if name != "" {
			names := []string{name}
			if strings.Contains(name, ",") {
				parts := strings.Split(name, ",")
				names = parts
			}

			profiles, err = svc.GetDeviceProfiles(ctx, names...)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		} else {
			profiles, err = svc.GetDeviceProfiles(ctx)
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		if len(profiles.Data) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var response ApiResponse

		if len(profiles.Data) == 1 {
			response = ApiResponse{
				Data: profiles.Data[0],
			}
		} else {
			meta := &meta{
				TotalRecords: profiles.TotalCount,
				Offset:       &profiles.Offset,
				Limit:        &profiles.Limit,
				Count:        profiles.Count,
			}
			response = ApiResponse{
				Data:  profiles.Data,
				Meta:  meta,
				Links: createLinks(r.URL, meta),
			}
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func queryLwm2mTypesHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "query-lwm2mtypes")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, _ = o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		urn := r.URL.Query().Get("urn")
		urnParam := chi.URLParam(r, "urn")

		if urn == "" && urnParam != "" {
			urn = urnParam
		}

		types, err := svc.GetLwm2mTypes(ctx, urn)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if len(types.Data) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var response ApiResponse

		if len(types.Data) == 1 {
			response = ApiResponse{
				Data: types.Data[0],
			}
		} else {
			meta := &meta{
				TotalRecords: types.TotalCount,
				Offset:       &types.Offset,
				Limit:        &types.Limit,
				Count:        types.Count,
			}
			response = ApiResponse{
				Data:  types.Data,
				Meta:  meta,
				Links: createLinks(r.URL, meta),
			}
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func isMultipartFormData(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "multipart/form-data")
}

func isApplicationJson(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json")
}

func wantsGeoJSON(r *http.Request) bool {
	contentType := r.Header.Get("Accept")
	return strings.Contains(contentType, "application/geo+json")
}

func wantsTextCSV(r *http.Request) bool {
	contentType := r.Header.Get("Accept")
	return strings.Contains(contentType, "text/csv")
}
