package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	k8shandlers "github.com/diwise/service-chassis/pkg/infrastructure/net/http/handlers"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/diwise/service-chassis/pkg/infrastructure/servicerunner"
	"go.yaml.in/yaml/v2"
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

	cfg, err := os.Open(flags[configurationFile])
	exitIf(err, logger, "could not open configuration file")

	dmCfg, err := parseExternalConfigFile(ctx, cfg)
	exitIf(err, logger, "could not create device management config")

	policies, err := os.Open(flags[policiesFile])
	exitIf(err, logger, "unable to open opa policy file")

	devices, err := os.Open(flags[devicesFile])
	exitIf(err, logger, "could not open devices file")

	runner, err := initialize(ctx, flags, dmCfg, policies, devices)
	exitIf(err, logger, "failed to initialize service runner")

	err = runner.Run(ctx)
	exitIf(err, logger, "failed to start service runner")
}

func initialize(ctx context.Context, flags flagMap, cfg *appConfig, policies, devices io.ReadCloser) (servicerunner.Runner[appConfig], error) {
	defer policies.Close()

	log := logging.GetFromContext(ctx)

	probes := map[string]k8shandlers.ServiceProber{
		"rabbitmq":  func(context.Context) (string, error) { return "ok", nil },
		"timescale": func(context.Context) (string, error) { return "ok", nil },
	}

	s, err := newStorage(ctx, flags)
	exitIf(err, log, "could not create or connect to database")

	messenger, err := messaging.Initialize(ctx, messaging.LoadConfiguration(ctx, serviceName, log))
	exitIf(err, log, "failed to init messenger")

	var dm devicemanagement.DeviceManagement
	var as alarms.AlarmService
	var wd watchdog.Watchdog

	_, runner := servicerunner.New(ctx, *cfg,
		webserver("control", listen(flags[listenAddress]), port(flags[controlPort]),
			pprof(), liveness(func() error { return nil }), readiness(probes),
		),
		webserver("public", listen(flags[listenAddress]), port(flags[servicePort]), tracing(flags[enableTracing] == "true"),
			muxinit(func(ctx context.Context, identifier string, port string, appCfg *appConfig, handler *http.ServeMux) error {
				return api.RegisterHandlers(ctx, handler, policies, dm, as, s)
			}),
		),
		oninit(func(ctx context.Context, ac *appConfig) error {
			log.Debug("initializing servicerunner")

			dm = devicemanagement.New(devicemanagement.NewStorage(s), messenger, &ac.DeviceManagementConfig)
			as = alarms.New(alarms.NewStorage(s), messenger, &ac.AlarmServiceConfig)
			wd = watchdog.New(as, &ac.WatchdogConfig)

			return nil
		}),
		onstarting(func(ctx context.Context, appCfg *appConfig) (err error) {
			log.Debug("starting servicerunner")

			err = s.Initialize(ctx)
			if err != nil {
				return
			}
			err = storage.SeedLwm2mTypes(ctx, s, appCfg.DeviceManagementConfig.Types)
			if err != nil {
				return
			}

			err = storage.SeedDeviceProfiles(ctx, s, appCfg.DeviceManagementConfig.DeviceProfiles)
			if err != nil {
				return
			}

			err = storage.SeedDevices(ctx, s, devices, strings.Split(flags[allowedSeedTenants], ","))
			if err != nil {
				return
			}

			messenger.Start()

			err = dm.RegisterTopicMessageHandler(ctx)
			if err != nil {
				return
			}

			err = as.RegisterTopicMessageHandler(ctx)
			if err != nil {
				return
			}

			wd.Start(ctx)

			return nil
		}),
		onshutdown(func(ctx context.Context, appCfg *appConfig) error {
			log.Debug("shutdown servicerunner")

			wd.Stop(ctx)
			messenger.Close()
			s.Close()

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

func parseExternalConfigFile(_ context.Context, cfgFile io.ReadCloser) (*appConfig, error) {
	defer cfgFile.Close()

	b, err := io.ReadAll(cfgFile)
	if err != nil {
		return nil, err
	}

	cfg := &appConfig{}
	err = yaml.Unmarshal(b, cfg)
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(cfg.DeviceManagementConfig.DeviceProfiles, func(dp types.DeviceProfile) bool {
		return dp.Decoder == "unknown"
	})

	if i < 0 {
		cfg.DeviceManagementConfig.DeviceProfiles = append(cfg.DeviceManagementConfig.DeviceProfiles, types.DeviceProfile{
			Name:    "unknown",
			Decoder: "unknown",
		})
	}

	return cfg, nil
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
