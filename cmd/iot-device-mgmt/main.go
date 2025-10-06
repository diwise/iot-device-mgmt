package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	k8shandlers "github.com/diwise/service-chassis/pkg/infrastructure/net/http/handlers"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/servicerunner"
)

const serviceName string = "iot-device-mgmt"

func defaultFlags() flagMap {
	return flagMap{
		listenAddress: "0.0.0.0",
		servicePort:   "8080",
		controlPort:   "8000",
		enableTracing: "true",

		policiesFile:      "/opt/diwise/config/authz.rego",
		configurationFile: "/opt/diwise/config/config.yaml",
		devicesFile:       "/opt/diwise/config/devices.csv",

		dbHost:     "",
		dbUser:     "",
		dbPassword: "",
		dbPort:     "5432",
		dbName:     "diwise",
		dbSSLMode:  "disable",

		updateExisitingDevices: "true",
		allowedSeedTenants:     "default",

		devmode: "false",
	}
}

func main() {
	ctx, flags := parseExternalConfig(context.Background(), defaultFlags())

	serviceVersion := buildinfo.SourceVersion()
	ctx, logger, cleanup := o11y.Init(ctx, serviceName, serviceVersion, "json")
	defer cleanup()

	storage, err := newStorage(ctx, flags)
	exitIf(err, logger, "could not create or connect to database")

	messenger, err := messaging.Initialize(ctx, messaging.LoadConfiguration(ctx, serviceName, logger))
	exitIf(err, logger, "failed to init messenger")

	policies, err := os.Open(flags[policiesFile])
	exitIf(err, logger, "unable to open opa policy file")

	cfg, err := os.Open(flags[configurationFile])
	exitIf(err, logger, "could not open configuration file")

	devices, err := os.Open(flags[devicesFile])
	exitIf(err, logger, "could not open devices file")

	dmCfg, err := devicemanagement.NewConfig(cfg)
	exitIf(err, logger, "could not create device management config")

	dm := devicemanagement.New(devicemanagement.NewStorage(storage), messenger, dmCfg)
	as := alarms.New(alarms.NewStorage(storage), messenger)
	wd := watchdog.New(as)

	appCfg := appConfig{
		messenger: messenger,
		db:        storage,
		dm:        dm,
		alarm:     as,
		watchdog:  wd,
	}

	runner, err := initialize(ctx, flags, &appCfg, policies, devices)
	exitIf(err, logger, "failed to initialize service runner")

	err = runner.Run(ctx)
	exitIf(err, logger, "failed to start service runner")
}

func initialize(ctx context.Context, flags flagMap, cfg *appConfig, policies, devices io.ReadCloser) (servicerunner.Runner[appConfig], error) {
	defer policies.Close()

	probes := map[string]k8shandlers.ServiceProber{
		"rabbitmq":  func(context.Context) (string, error) { return "ok", nil },
		"timescale": func(context.Context) (string, error) { return "ok", nil },
	}

	_, runner := servicerunner.New(ctx, *cfg,
		webserver("control", listen(flags[listenAddress]), port(flags[controlPort]),
			pprof(), liveness(func() error { return nil }), readiness(probes),
		),
		webserver("public", listen(flags[listenAddress]), port(flags[servicePort]), tracing(flags[enableTracing] == "true"),
			muxinit(func(ctx context.Context, identifier string, port string, appCfg *appConfig, handler *http.ServeMux) error {
				return api.RegisterHandlers(ctx, handler, policies, appCfg.dm, appCfg.alarm, appCfg.db)
			}),
		),
		onstarting(func(ctx context.Context, appCfg *appConfig) (err error) {
			err = appCfg.db.Initialize(ctx)
			if err != nil {
				return
			}

			err = storage.SeedLwm2mTypes(ctx, appCfg.db, appCfg.dm.Config().Types)
			if err != nil {
				return
			}

			err = storage.SeedDeviceProfiles(ctx, appCfg.db, appCfg.dm.Config().DeviceProfiles)
			if err != nil {
				return
			}

			err = storage.SeedDevices(ctx, appCfg.db, devices, strings.Split(flags[allowedSeedTenants], ","))
			if err != nil {
				return
			}

			appCfg.messenger.Start()

			err = appCfg.dm.RegisterTopicMessageHandler(ctx)
			if err != nil {
				return
			}

			err = appCfg.alarm.RegisterTopicMessageHandler(ctx)
			if err != nil {
				return
			}

			appCfg.watchdog.Start(ctx)

			return nil
		}),
		onshutdown(func(ctx context.Context, appCfg *appConfig) error {
			appCfg.watchdog.Stop(ctx)
			appCfg.messenger.Close()
			appCfg.db.Close()

			return nil
		}),
	)

	return runner, nil
}

func newStorage(ctx context.Context, flags flagMap) (storage.Store, error) {

	exisitingDeviceUpdateFlag, _ := strconv.ParseBool(flags[updateExisitingDevices])

	if flags[devmode] == "true" {
		return &storage.StoreMock{}, fmt.Errorf("not implemented")
	}
	return storage.New(ctx, storage.NewConfig(flags[dbHost], flags[dbUser], flags[dbPassword], flags[dbPort], flags[dbName],
		flags[dbSSLMode], exisitingDeviceUpdateFlag))
}

func parseExternalConfig(ctx context.Context, flags flagMap) (context.Context, flagMap) {
	// Allow environment variables to override certain defaults
	envOrDef := env.GetVariableOrDefault

	flags[listenAddress] = envOrDef(ctx, "LISTEN_ADDRESS", flags[listenAddress])
	flags[controlPort] = envOrDef(ctx, "CONTROL_PORT", flags[controlPort])
	flags[servicePort] = envOrDef(ctx, "SERVICE_PORT", flags[servicePort])

	flags[policiesFile] = envOrDef(ctx, "POLICIES_FILE", flags[policiesFile])
	flags[allowedSeedTenants] = envOrDef(ctx, "ALLOWED_SEED_TENANTS", flags[allowedSeedTenants])

	flags[dbHost] = envOrDef(ctx, "POSTGRES_HOST", flags[dbHost])
	flags[dbPort] = envOrDef(ctx, "POSTGRES_PORT", flags[dbPort])
	flags[dbName] = envOrDef(ctx, "POSTGRES_DBNAME", flags[dbName])
	flags[dbUser] = envOrDef(ctx, "POSTGRES_USER", flags[dbUser])
	flags[dbPassword] = envOrDef(ctx, "POSTGRES_PASSWORD", flags[dbPassword])
	flags[dbSSLMode] = envOrDef(ctx, "POSTGRES_SSLMODE", flags[dbSSLMode])
	flags[enableTracing] = envOrDef(ctx, "ENABLE_TRACING", flags[enableTracing])
	flags[updateExisitingDevices] = envOrDef(ctx, "UPDATE_EXISTING_DEVICES", flags[updateExisitingDevices])

	apply := func(f flagType) func(string) error {
		return func(value string) error {
			flags[f] = value
			return nil
		}
	}

	// Allow command line arguments to override defaults and environment variables
	flag.Func("policies", "an authorization policy file", apply(policiesFile))
	flag.Func("devices", "list of known devices", apply(devicesFile))
	flag.Func("config", "device management configuration file", apply(configurationFile))
	flag.Func("devmode", "enable dev mode", apply(devmode))
	flag.Parse()

	return ctx, flags
}

func exitIf(err error, logger *slog.Logger, msg string, args ...any) {
	if err != nil {
		logger.With(args...).Error(msg, "err", err.Error())
		time.Sleep(2 * time.Second)
		os.Exit(1)
	}
}
