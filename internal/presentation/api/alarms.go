package api

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api/auth"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func getAlarmsHandler(log *slog.Logger, svc alarms.AlarmAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetTenantsWithAllowedScopes(r.Context(), ReadDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(r.Context(), "get-alarms")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, _ = o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		query, parseErr := alarmQueryFromValues(r.URL.Query(), allowedTenants)
		if parseErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(parseErr.Error()))
			return
		}

		result, err := svc.Alarms(ctx, query)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		meta := &meta{TotalRecords: result.TotalCount, Offset: &result.Offset, Limit: &result.Limit, Count: result.Count}
		response := ApiResponse{Data: result.Data, Meta: meta, Links: createLinks(r.URL, meta)}

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}
