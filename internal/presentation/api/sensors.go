package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/application/sensormanagement"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/tracing"
)

func querySensorsHandler(log *slog.Logger, svc sensormanagement.SensorAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "query-sensors")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		query, parseErr := sensorQueryFromValues(r.URL.Query())
		if parseErr != nil {
			logger.Error("invalid sensor query", "err", parseErr.Error())
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(parseErr.Error()))
			return
		}

		result, err := svc.Query(ctx, query)
		if err != nil {
			logger.Error("could not query sensors", "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
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

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func getSensorHandler(log *slog.Logger, svc sensormanagement.SensorAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "get-sensor")
		defer func() { tracing.RecordAnyErrorAndEndSpan(err, span) }()
		_, ctx, logger := o11y.AddTraceIDToLoggerAndStoreInContext(span, log, ctx)

		sensorID := r.PathValue("id")
		if sensorID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		ctx = logging.NewContextWithLogger(ctx, logger, slog.String("sensor_id", sensorID))

		sensor, err := svc.Sensor(ctx, sensorID)
		if err != nil {
			if errors.Is(err, sensormanagement.ErrSensorNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("could not fetch sensor", "sensor_id", sensorID, "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		response := ApiResponse{Data: sensor}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response.Byte())
	}
}

func createSensorHandler(log *slog.Logger, svc sensormanagement.SensorAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "create-sensor")
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

		var sensor sensormanagement.Sensor
		err = json.Unmarshal(body, &sensor)
		if err != nil {
			logger.Error("unable to unmarshal body", "body", string(body), "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if sensor.SensorID == "" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.Create(ctx, sensor)
		if err != nil {
			if errors.Is(err, sensormanagement.ErrSensorAlreadyExists) {
				w.WriteHeader(http.StatusConflict)
				return
			}

			logger.Error("unable to create sensor", "sensor_id", sensor.SensorID, "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
	}
}

func updateSensorHandler(log *slog.Logger, svc sensormanagement.SensorAPIService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error

		ctx, span := tracer.Start(r.Context(), "update-sensor")
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

		var sensor sensormanagement.Sensor
		err = json.Unmarshal(body, &sensor)
		if err != nil {
			logger.Error("unable to unmarshal body", "body", string(body), "err", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		id := r.PathValue("id")
		if id == "" || sensor.SensorID == "" || id != sensor.SensorID {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		err = svc.Update(ctx, sensor)
		if err != nil {
			if errors.Is(err, sensormanagement.ErrSensorNotFound) {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			logger.Error("unable to update sensor", "sensor_id", sensor.SensorID, "err", err.Error())
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}
