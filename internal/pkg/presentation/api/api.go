package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

type CollectionResponse struct {
	Meta  meta  `json:"meta"`
	Data  any   `json:"data"`
	Links links `json:"links"`
}

func NewCollectionResponse(total, offset, limit, count uint64, results any, baseUrl string) *CollectionResponse {
	r := &CollectionResponse{
		Meta: meta{
			TotalRecords: total,
			Count:        count,
		},
		Data: results,
	}

	if total == 1 && offset == 1 && limit == 1 && count == 1 {
		r.Links.Self = &baseUrl
	} else {
		r.Meta.Offset = &offset
		r.Meta.Limit = &limit

		// self, always
		if total > 1 {
			link := baseUrl + fmt.Sprintf("page[offset]=%d&page[limit]=%d", offset, limit)
			r.Links.Self = &link
		} else {
			r.Links.Self = &baseUrl
		}

		// first if total > 0
		if total > 0 {
			link := baseUrl + fmt.Sprintf("page[offset]=0&page[limit]=%d", limit)
			r.Links.First = &link
		}

		// prev if offset > 0
		if offset > 0 {
			newOffset := offset - limit
			newLimit := limit
			if offset > limit {
				newOffset = 0
				newLimit = offset
			}
			link := baseUrl + fmt.Sprintf("page[offset]=%d&page[limit]=%d", newOffset, newLimit)
			r.Links.Prev = &link
		}

		// next if offset+limit < total
		if offset+limit < total {
			link := baseUrl + fmt.Sprintf("page[offset]=%d&page[limit]=%d", offset+limit, limit)
			r.Links.Next = &link
		}

		// last if total > 0
		if total > 0 {
			newOffset := total - limit
			if total < limit {
				newOffset = 0
			}
			link := baseUrl + fmt.Sprintf("page[offset]=%d&page[limit]=%d", newOffset, limit)
			r.Links.Last = &link
		}
	}

	return r
}

func getProtocolAndHostFromRequest(ctx context.Context, r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if proto == "" {
		proto = "http"
	}

	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}

	return fmt.Sprintf("%s://%s", proto, host)
}

func getOffsetAndLimitFromRequest(ctx context.Context, r *http.Request) (uint64, uint64, error) {

	offset := uint64(0)
	limit := uint64(50)
	var err error

	limitParam := r.URL.Query().Get("page[limit]")
	if limitParam == "" {
		limitParam = r.URL.Query().Get("limit")
	}
	if limitParam != "" {
		limit, err = strconv.ParseUint(limitParam, 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid limit parameter in query")
		}
		if limit < 10 {
			limit = 10
		} else if limit > 100 {
			limit = 100
		}
	}

	offsetParam := r.URL.Query().Get("page[offset]")
	if offsetParam == "" {
		offsetParam = r.URL.Query().Get("offset")
		if offsetParam == "" {
			page := r.URL.Query().Get("page")
			if page != "" {
				offset, err = strconv.ParseUint(page, 10, 64)
				if err != nil || offset == 0 {
					return 0, 0, errors.New("invalid page parameter in query")
				}
				offset = (offset - 1) * limit
				offsetParam = strconv.FormatUint(offset, 10)
			}
		}
	}
	if offsetParam != "" {
		offset, err = strconv.ParseUint(offsetParam, 10, 64)
		if err != nil {
			return 0, 0, errors.New("invalid offset parameter in query")
		}
	}

	return offset, limit, nil
}

func createDeviceHandler(log zerolog.Logger, svc devicemanagement.DeviceManagement) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		defer r.Body.Close()

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

		err = svc.CreateDevice(ctx, d)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to create device")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
	}
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

		offset, limit, err := getOffsetAndLimitFromRequest(ctx, r)
		host := getProtocolAndHostFromRequest(ctx, r)
		var result *CollectionResponse

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
				requestLogger.Error().Err(err).Msg("unable to map device")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			devices = append(devices, d)
			result = NewCollectionResponse(1, 1, 1, 1, devices, host+"/api/v0/devices?devEUI="+sensorID)
		} else {
			totalCount, fromDb, err := svc.GetDevices(ctx, offset, limit, allowedTenants...)
			if err != nil {
				requestLogger.Error().Err(err).Msg("unable to fetch all devices")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			for _, device := range fromDb {
				d, err := devicemanagement.MapTo[types.Device](device)
				if err != nil {
					requestLogger.Error().Err(err).Msg("unable to map device")
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				devices = append(devices, d)
			}

			result = NewCollectionResponse(totalCount, offset, limit, uint64(len(devices)), devices, host+"/api/v0/devices?")
		}

		b, err := json.Marshal(result)
		if err != nil {
			requestLogger.Error().Err(err).Msg("unable to marshal result")
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
