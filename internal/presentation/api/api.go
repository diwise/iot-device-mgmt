package api

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/diwise/iot-device-mgmt/internal/application"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api/auth"

	"github.com/diwise/service-chassis/pkg/infrastructure/net/http/router"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"

	"go.opentelemetry.io/otel"
)

var tracer = otel.Tracer("iot-device-mgmt/api")

func RegisterHandlers(ctx context.Context, mux *http.ServeMux, policies io.Reader, app application.Management) error {
	const apiPrefix string = "/api/v0"

	log := logging.GetFromContext(ctx)

	authenticator, err := auth.NewAuthenticator(ctx, policies)
	if err != nil {
		return fmt.Errorf("failed to create api authenticator: %w", err)
	}

	r := router.New(mux, router.WithPrefix(apiPrefix))

	r.Use(authenticator)

	r.Get("/sensors", querySensorsHandler(log, app.SensorService()))
	r.Get("/sensors/{id}", getSensorHandler(log, app.SensorService()))
	r.Post("/sensors", createSensorHandler(log, app.SensorService()))
	r.Put("/sensors/{id}", updateSensorHandler(log, app.SensorService()))

	r.Get("/devices", queryDevicesHandler(log, app.DeviceService()))
	r.Get("/devices/{id}", getDeviceHandler(log, app.DeviceService()))
	r.Get("/devices/{id}/status", getDeviceStatusHandler(log, app.DeviceService()))
	r.Get("/devices/{id}/alarms", getDeviceAlarmsHandler(log, app.DeviceService()))
	r.Get("/devices/{id}/measurements", getDeviceMeasurementsHandler(log, app.DeviceService()))

	r.Post("/devices", createDeviceHandler(log, app)) //TODO: fix import endpoint to use device service directly instead of seeding method
	r.Put("/devices/{id}", updateDeviceHandler(log, app.DeviceService()))
	r.Patch("/devices/{id}", patchDeviceHandler(log, app.DeviceService()))
	r.Put("/devices/{id}/sensor", attachDeviceSensorHandler(log, app.DeviceService()))
	r.Delete("/devices/{id}/sensor", detachDeviceSensorHandler(log, app.DeviceService()))

	r.Get("/alarms", getAlarmsHandler(log, app.AlarmService()))

	r.Get("/admin/deviceprofiles", queryDeviceProfilesHandler(log, app.DeviceService()))
	r.Get("/admin/deviceprofiles/{id}", queryDeviceProfilesHandler(log, app.DeviceService()))
	r.Get("/admin/lwm2mtypes", queryLwm2mTypesHandler(log, app.DeviceService()))
	r.Get("/admin/lwm2mtypes/{urn}", queryLwm2mTypesHandler(log, app.DeviceService()))
	r.Get("/admin/tenants", queryTenantsHandler())

	return nil
}
