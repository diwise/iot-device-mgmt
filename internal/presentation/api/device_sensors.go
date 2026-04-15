package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/application/devices"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api/auth"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

type attachSensorRequest struct {
	SensorID string `json:"sensorID"`
}

func attachDeviceSensorHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetTenantsWithAllowedScopes(r.Context(), UpdateDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(r.Context(), "attach-device-sensor")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		if !isApplicationJson(r) {
			logger.Error("Unsupported MediaType")
			w.WriteHeader(http.StatusUnsupportedMediaType)
			return
		}

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		body, err := io.ReadAll(r.Body)
		if err != nil {
			logger.Error("unable to read body", "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var request attachSensorRequest
		err = json.Unmarshal(body, &request)
		if err != nil || request.SensorID == "" {
			logger.Error("unable to unmarshal attach sensor request", "body", string(body), "err", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.AttachSensor(ctx, deviceID, request.SensorID, allowedTenants)
		if err != nil {
			switch {
			case errors.Is(err, devices.ErrDeviceNotFound), errors.Is(err, devices.ErrSensorNotFound):
				w.WriteHeader(http.StatusNotFound)
			case errors.Is(err, devices.ErrSensorAlreadyAssigned), errors.Is(err, devices.ErrSensorProfileRequired):
				w.WriteHeader(http.StatusConflict)
			default:
				logger.Error("unable to attach sensor", "sensor_id", request.SensorID, "err", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}

func detachDeviceSensorHandler(log *slog.Logger, svc devices.DeviceAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		allowedTenants := auth.GetTenantsWithAllowedScopes(r.Context(), UpdateDevices)
		if len(allowedTenants) == 0 {
			err = errors.New("not authorized")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		ctx, span := tracer.Start(r.Context(), "detach-device-sensor")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		deviceID := r.PathValue("id")
		if deviceID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("device_id", deviceID))

		err = svc.DetachSensor(ctx, deviceID, allowedTenants)
		if err != nil {
			if errors.Is(err, devices.ErrDeviceNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("unable to detach sensor", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
