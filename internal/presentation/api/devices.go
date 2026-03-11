package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/application"
	"github.com/diwise/iot-device-mgmt/internal/application/devices"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api/auth"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func queryDevicesHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "query-devices")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		sensorID := r.URL.Query().Get("devEUI")

		if sensorID != "" {
			ctx = logging.NewContextWithLogger(ctx, logger, slog.String("sensor_id", sensorID))

			logger.Debug(fmt.Sprintf("request devEUI %s for tenants %s", sensorID, strings.Join(allowedTenants, ", ")))

			device, err := svc.DeviceBySensor(ctx, sensorID, allowedTenants)
			if err != nil {
				if errors.Is(err, devices.ErrDeviceNotFound) {
					logger.Debug(fmt.Sprintf("device %s not found", sensorID))
					w.WriteHeader(http.StatusNotFound)
					return
				}

				logger.Error("could not fetch data", "sensor_id", sensorID, "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			response := ApiResponse{Data: device}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response.Byte())

			return
		}

		query, parseErr := deviceQueryFromValues(r.URL.Query(), allowedTenants)
		if parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(parseErr.Error()))
			return
		}

		collection, err := svc.Query(ctx, query)
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
	}
}

func getDeviceHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		device, err := svc.Device(ctx, deviceID, allowedTenants)
		if err != nil {
			if errors.Is(err, devices.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{Data: device}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func getDeviceStatusHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "get-device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		allowedTenants := auth.GetAllowedTenantsFromContext(ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		query, parseErr := deviceStatusQueryFromValues(r.URL.Query(), allowedTenants)
		if parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(parseErr.Error()))
			return
		}

		statuses, err := svc.Status(ctx, deviceID, query)
		if err != nil {
			if errors.Is(err, devices.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{
			Data: statuses.Data,
			Meta: &meta{TotalRecords: statuses.TotalCount, Count: statuses.Count},
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func getDeviceAlarmsHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		alarmDetails, err := svc.Alarms(ctx, deviceID, allowedTenants)
		if err != nil {
			if errors.Is(err, devices.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{
			Data: alarmDetails.Data,
			Meta: &meta{TotalRecords: alarmDetails.TotalCount, Count: alarmDetails.Count},
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func getDeviceMeasurementsHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "get-device-status")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		query, parseErr := deviceMeasurementsQueryFromValues(r.URL.Query(), allowedTenants)
		if parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(parseErr.Error()))
			return
		}

		result, err := svc.Measurements(ctx, deviceID, query)
		if err != nil {
			if errors.Is(err, devices.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch device details", slog.String("device_id", deviceID), "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{
			Data: result.Data,
			Meta: &meta{TotalRecords: result.TotalCount, Offset: &result.Offset, Limit: &result.Limit, Count: result.Count},
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func createDeviceHandler(log *slog.Logger, app application.Management) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "create-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		if isMultipartFormData(r) {
			file, _, err := r.FormFile("fileupload")
			if err != nil {
				logger.Error("unable to read file", "err", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			err = app.SeedSensorsAndDevices(ctx, file, allowedTenants, true)
			if err != nil {
				logger.Error("failed to import data", "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			return
		}

		if isApplicationJson(r) {
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

			err = app.DeviceService().Create(ctx, d)
			if err != nil {
				if errors.Is(err, devices.ErrDeviceAlreadyExist) {
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

func updateDeviceHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "update-device")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		if !isApplicationJson(r) {
			logger.Error("Unsupported MediaType")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
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

		err = svc.Update(ctx, d)
		if err != nil {
			if errors.Is(err, devices.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			logger.Error("unable to create device", "device_id", d.DeviceID, "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

func patchDeviceHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		allowedTenants := auth.GetAllowedTenantsFromContext(r.Context())

		ctx, span := tracer.Start(r.Context(), "patch-device")
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

		err = svc.Merge(ctx, deviceID, fields, allowedTenants)
		if err != nil {
			if errors.Is(err, devices.ErrInvalidPatch) {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if errors.Is(err, devices.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			logger.Error("unable to update device", "device_id", deviceID, "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}
