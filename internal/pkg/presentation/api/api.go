package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api/auth"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/router"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

const (
	CreateDevices auth.Scope = auth.Scope("devices.create")
	ReadDevices   auth.Scope = auth.Scope("devices.read")
	UpdateDevices auth.Scope = auth.Scope("devices.update")
)

func RegisterHandlers(ctx context.Context, mux *http.ServeMux, policies io.Reader, dm devicemanagement.DeviceManagement, alarm alarms.AlarmService, s storage.Store) error {
	const apiPrefix string = "/api/v0"

	log := logging.GetFromContext(ctx)

	authz, err := auth.NewAuthenticator(ctx, policies)
	if err != nil {
		return fmt.Errorf("failed to create api authenticator: %w", err)
	}

	r := router.New(mux, router.WithPrefix(apiPrefix), router.WithTaggedRoutes(true))

	r.Route("/devices", func(r router.ServeMux) {
		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(ReadDevices))
			r.Group(func(r router.ServeMux) {
				r.Get("", queryDevicesHandler(log, dm))
				r.Get("/{id}", getDeviceHandler(log, dm))
				r.Get("/{id}/status", getDeviceStatusHandler(log, dm))
				r.Get("/{id}/alarms", getDeviceAlarmsHandler(log, dm))
				r.Get("/{id}/measurements", getDeviceMeasurementsHandler(log, dm))
			})
		})

		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(UpdateDevices))
			r.Group(func(r router.ServeMux) {
				r.Put("/{id}", updateDeviceHandler(log, dm))
				r.Patch("/{id}", patchDeviceHandler(log, dm))
			})
		})

		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(CreateDevices))
			r.Group(func(r router.ServeMux) {
				r.Post("", createDeviceHandler(log, dm, s))
			})
		})
	})

	r.Route("/alarms", func(r router.ServeMux) {
		r.Use(authz.RequireAccess(ReadDevices))
		r.Get("", getAlarmsHandler(log, alarm))
	})

	r.Route("/admin", func(r router.ServeMux) {
		r.Get("/deviceprofiles", queryDeviceProfilesHandler(log, dm))
		r.Get("/deviceprofiles/{id}", queryDeviceProfilesHandler(log, dm))
		r.Get("/lwm2mtypes", queryLwm2mTypesHandler(log, dm))
		r.Get("/lwm2mtypes/{urn}", queryLwm2mTypesHandler(log, dm))
		r.Get("/tenants", queryTenantsHandler())
	})

	return nil
}

func queryDevicesHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "query-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetTenantsWithAllowedScopes(r.Context(), ReadDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		sensorID := r.URL.Query().Get("devEUI")

		if sensorID != "" {
			ctx = logging.NewContextWithLogger(ctx, logger, slog.String("sensor_id", sensorID))

			logger.Debug(fmt.Sprintf("request devEUI %s for tenants %s", sensorID, strings.Join(allowedTenants, ", ")))

			device, err := svc.GetBySensorID(ctx, sensorID, allowedTenants)
			if err != nil {
				if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
					logger.Debug(fmt.Sprintf("device %s not found", sensorID))
					w.WriteHeader(http.StatusNotFound)
					return
				}

				logger.Error("could not fetch data", "sensor_id", sensorID, "err", err.Error())
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

func getDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, ReadDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		device, err := svc.GetByDeviceID(ctx, deviceID, allowedTenants)
		if err != nil {
			if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
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

func getDeviceStatusHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, ReadDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(ctx, "get-device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		statuses, err := svc.GetDeviceStatus(ctx, deviceID, r.URL.Query(), allowedTenants)
		if err != nil {
			if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{
			Data: statuses.Data,
			Meta: &meta{
				TotalRecords: statuses.TotalCount,
				Count:        statuses.Count,
			},
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func getDeviceAlarmsHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, ReadDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(ctx, "get-device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		alarms, err := svc.GetDeviceAlarms(ctx, deviceID, allowedTenants)
		if err != nil {
			if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{
			Data: alarms.Data,
			Meta: &meta{
				TotalRecords: alarms.TotalCount,
				Count:        alarms.Count,
			},
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func getDeviceMeasurementsHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, ReadDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(ctx, "get-device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		result, err := svc.GetDeviceMeasurements(ctx, deviceID, r.URL.Query(), allowedTenants)
		if err != nil {
			if errors.Is(err, devicemanagement.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{
			Data: result.Data,
			Meta: &meta{
				TotalRecords: result.TotalCount,
				Offset:       &result.Offset,
				Limit:        &result.Limit,
				Count:        result.Count,
			},
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func createDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement, s storage.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, CreateDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(ctx, "create-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		if isMultipartFormData(r) {
			file, _, err := r.FormFile("fileupload")
			if err != nil {
				logger.Error("unable to read file", "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = storage.SeedDevices(ctx, s, file, allowedTenants)
			if err != nil {
				logger.Error("failed to import data", "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			return
		} else if isApplicationJson(r) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				logger.Error("unable to read body", "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			var d types.Device
			err = json.Unmarshal(body, &d)
			if err != nil {
				logger.Error("unable to unmarshal body", "body", string(body), "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if !slices.Contains(allowedTenants, d.Tenant) {
				logger.Error("not allowed to create device with current tenant", "device_id", d.DeviceID, "tenant", d.Tenant)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			err = svc.NewDevice(ctx, d)
			if err != nil {
				if errors.Is(err, devicemanagement.ErrDeviceAlreadyExist) {
					w.WriteHeader(http.StatusConflict)
					return
				}

				logger.Error("unable to create device", "device_id", d.DeviceID, "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			return
		}

		logger.Error("Unsupported MediaType")
		w.WriteHeader(http.StatusUnsupportedMediaType)
	}
}

func updateDeviceHandler(log *slog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, UpdateDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(ctx, "update-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		if !isApplicationJson(r) {
			logger.Error("Unsupported MediaType")
			w.WriteHeader(http.StatusUnsupportedMediaType)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("unable to read body", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var d types.Device
		err = json.Unmarshal(body, &d)
		if err != nil {
			logger.Error("unable to unmarshal body", "body", string(body), "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !slices.Contains(allowedTenants, d.Tenant) {
			logger.Error("not allowed to update device with current tenant", "device_id", d.DeviceID, "tenant", d.Tenant)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		id := r.PathValue("id")
		if id != d.DeviceID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.UpdateDevice(ctx, d)
		if err != nil {
			logger.Error("unable to create device", "device_id", d.DeviceID, "err", err.Error())
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
		ctx := r.Context()

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, UpdateDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(ctx, "patch-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")

		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		b, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("unable to read body", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var fields map[string]any
		err = json.Unmarshal(b, &fields)
		if err != nil {
			logger.Error("unable to unmarshal body into map", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.MergeDevice(ctx, deviceID, fields, allowedTenants)
		if err != nil {
			logger.Error("unable to update device", "device_id", deviceID, "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
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
		deviceprofileId := r.PathValue("id")

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
		urnParam := r.PathValue("urn")

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

func getAlarmsHandler(log *slog.Logger, svc alarms.AlarmService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		allowedTenants := auth.GetTenantsWithAllowedScopes(ctx, ReadDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(ctx, "get-alarms")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, _ = o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		result, err := svc.GetAlarms(ctx, r.URL.Query(), allowedTenants)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		meta := &meta{
			TotalRecords: result.TotalCount,
			Offset:       &result.Offset,
			Limit:        &result.Limit,
			Count:        result.Count,
		}
		response := ApiResponse{
			Data:  result.Data,
			Meta:  meta,
			Links: createLinks(r.URL, meta),
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

/* -------------- --------------- --------------- */

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

func queryTenantsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetTenantsWithAllowedScopes(r.Context(), ReadDevices)

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
