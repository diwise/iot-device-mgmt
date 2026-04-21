package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/application/devices"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api/auth"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func queryDeviceProfilesHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
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

		var profiles types.Collection[types.SensorProfile]

		if name != "" {
			names := []string{name}
			if strings.Contains(name, ",") {
				names = strings.Split(name, ",")
			}

			profiles, err = svc.Profiles(ctx, names...)
			if err != nil {
				if errors.Is(err, devices.ErrDeviceProfileNotFound) {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		} else {
			profiles, err = svc.Profiles(ctx)
			if err != nil {
				if errors.Is(err, devices.ErrDeviceProfileNotFound) {
					w.WriteHeader(http.StatusNotFound)
					return
				}

				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

		if len(profiles.Data) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var response ApiResponse

		if len(profiles.Data) == 1 {
			response = ApiResponse{Data: profiles.Data[0]}
		} else {
			meta := &meta{TotalRecords: profiles.TotalCount, Offset: &profiles.Offset, Limit: &profiles.Limit, Count: profiles.Count}
			response = ApiResponse{Data: profiles.Data, Meta: meta, Links: createLinks(r.URL, meta)}
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func queryLwm2mTypesHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
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

		profileTypes, err := svc.Lwm2mTypes(ctx, urn)
		if err != nil {
			if errors.Is(err, devices.ErrDeviceProfileNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if len(profileTypes.Data) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var response ApiResponse

		if len(profileTypes.Data) == 1 {
			response = ApiResponse{Data: profileTypes.Data[0]}
		} else {
			meta := &meta{TotalRecords: profileTypes.TotalCount, Offset: &profileTypes.Offset, Limit: &profileTypes.Limit, Count: profileTypes.Count}
			response = ApiResponse{Data: profileTypes.Data, Meta: meta, Links: createLinks(r.URL, meta)}
		}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func queryTenantsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetTenantsWithAllowedScopes(r.Context(), auth.AnyScope)

		_, span := tracer.Start(r.Context(), "query-tenants")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()

		response := ApiResponse{Data: allowedTenants}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}
