package api

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/application/sensormanagement"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api/auth"

	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/router"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(ctx context.Context, mux *http.ServeMux, policies io.Reader, dm devicemanagement.DeviceAPIService, sm sensormanagement.SensorAPIService, alarm alarms.AlarmService) error {
	const apiPrefix string = "/api/v0"

	log := logging.GetFromContext(ctx)

	authenticator, err := auth.NewAuthenticator(ctx, policies)
	if err != nil {
		return fmt.Errorf("failed to create api authenticator: %w", err)
	}

	r := router.New(mux, router.WithPrefix(apiPrefix))

	r.Use(authenticator)

	r.Get("/sensors", querySensorsHandler(log, sm))
	r.Get("/sensors/{id}", getSensorHandler(log, sm))
	r.Post("/sensors", createSensorHandler(log, sm))
	r.Put("/sensors/{id}", updateSensorHandler(log, sm))

	r.Get("/devices", queryDevicesHandler(log, dm))
	r.Get("/devices/{id}", getDeviceHandler(log, dm))
	r.Get("/devices/{id}/status", getDeviceStatusHandler(log, dm))
	r.Get("/devices/{id}/alarms", getDeviceAlarmsHandler(log, dm))
	r.Get("/devices/{id}/measurements", getDeviceMeasurementsHandler(log, dm))

	r.Post("/devices", createDeviceHandler(log, dm))
	r.Put("/devices/{id}", updateDeviceHandler(log, dm))
	r.Patch("/devices/{id}", patchDeviceHandler(log, dm))
	r.Put("/devices/{id}/sensor", attachDeviceSensorHandler(log, dm))
	r.Delete("/devices/{id}/sensor", detachDeviceSensorHandler(log, dm))

	r.Get("/alarms", getAlarmsHandler(log, alarm))

	r.Get("/admin/deviceprofiles", queryDeviceProfilesHandler(log, dm))
	r.Get("/admin/deviceprofiles/{id}", queryDeviceProfilesHandler(log, dm))
	r.Get("/admin/lwm2mtypes", queryLwm2mTypesHandler(log, dm))
	r.Get("/admin/lwm2mtypes/{urn}", queryLwm2mTypesHandler(log, dm))
	r.Get("/admin/tenants", queryTenantsHandler())

	return nil
}
