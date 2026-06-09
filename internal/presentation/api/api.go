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

const (
	CreateDevices auth.Scope = auth.Scope("devices.create")
	ReadDevices   auth.Scope = auth.Scope("devices.read")
	UpdateDevices auth.Scope = auth.Scope("devices.update")

	CreateSensors auth.Scope = auth.Scope("sensors.create")
	ReadSensors   auth.Scope = auth.Scope("sensors.read")
	UpdateSensors auth.Scope = auth.Scope("sensors.update")
)

func RegisterHandlers(ctx context.Context, mux *http.ServeMux, policies io.Reader, app application.Management) error {
	const apiPrefix string = "/api/v0"

	log := logging.GetFromContext(ctx)

	authz, err := auth.NewAuthenticator(ctx, policies)
	if err != nil {
		return fmt.Errorf("failed to create api authenticator: %w", err)
	}

	r := router.New(mux, router.WithPrefix(apiPrefix))

	r.Route("sensors", func(r router.ServeMux) {
		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(ReadSensors))
			r.Group(func(r router.ServeMux) {
				r.Get("", querySensorsHandler(log, app.SensorService()))
				r.Get("{id}", getSensorHandler(log, app.SensorService()))
			})
		})

		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(UpdateSensors))
			r.Put("{id}", updateSensorHandler(log, app.SensorService()))
		})

		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(CreateSensors))
			r.Post("", createSensorHandler(log, app.SensorService()))
		})
	})

	r.Route("devices", func(r router.ServeMux) {
		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(ReadDevices))
			r.Group(func(r router.ServeMux) {
				r.Get("", queryDevicesHandler(log, app.DeviceService()))
				r.Get("{id}", getDeviceHandler(log, app.DeviceService()))
				r.Get("{id}/status", getDeviceStatusHandler(log, app.DeviceService()))
				r.Get("{id}/alarms", getDeviceAlarmsHandler(log, app.DeviceService()))
				r.Get("{id}/measurements", getDeviceMeasurementsHandler(log, app.DeviceService()))
			})
		})

		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(UpdateDevices))
			r.Group(func(r router.ServeMux) {
				r.Put("{id}", updateDeviceHandler(log, app.DeviceService()))
				r.Patch("{id}", patchDeviceHandler(log, app.DeviceService()))
				r.Put("{id}/sensor", attachDeviceSensorHandler(log, app.DeviceService()))
				r.Delete("{id}/sensor", detachDeviceSensorHandler(log, app.DeviceService()))
			})
		})

		r.Group(func(r router.ServeMux) {
			r.Use(authz.RequireAccess(CreateDevices))
			r.Post("", createDeviceHandler(log, app)) //TODO: fix import endpoint to use device service directly instead of seeding method
		})
	})

	r.Route("alarms", func(r router.ServeMux) {
		r.Use(authz.RequireAccess(ReadDevices))
		r.Get("", getAlarmsHandler(log, app.AlarmService()))
	})

	r.Route("admin", func(r router.ServeMux) {
		r.Get("deviceprofiles", queryDeviceProfilesHandler(log, app.DeviceService()))
		r.Get("deviceprofiles/{id}", queryDeviceProfilesHandler(log, app.DeviceService()))
		r.Get("lwm2mtypes", queryLwm2mTypesHandler(log, app.DeviceService()))
		r.Get("lwm2mtypes/{urn}", queryLwm2mTypesHandler(log, app.DeviceService()))
		r.Get("tenants", queryTenantsHandler())
	})

	return nil
}
