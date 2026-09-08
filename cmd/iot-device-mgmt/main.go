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
	"sync"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/application"
	"github.com/diwise/iot-device-mgmt/internal/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/application/devices"
	"github.com/diwise/iot-device-mgmt/internal/application/sensors"
	"github.com/diwise/iot-device-mgmt/internal/application/watchdog"
	"github.com/diwise/iot-device-mgmt/internal/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api"
	"github.com/diwise/iot-device-mgmt/internal/presentation/api/auth"
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
		authzAccessObject: "false",
		configurationFile: "/opt/diwise/config/config.yaml",
		devicesFile:       "/opt/diwise/config/devices.csv",

		dbHost:     "",
		dbUser:     "",
		dbPassword: "",
		dbPort:     "5432",
		dbName:     "diwise",
		dbSSLMode:  "disable",

		seedExistingDevices: "true",
		allowedSeedTenants:  "default",

		devmode: "false",

		logLevel: "debug",
	}
}

func main() {
	ctx, flags := parseExternalConfig(context.Background(), defaultFlags())

	serviceVersion := buildinfo.SourceVersion()
	ctx, logger, cleanup := o11y.Init(ctx, serviceName, serviceVersion, "json")
	defer cleanup()

	logging.SetLogLevel(parseLogLevel(flags[logLevel]))

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

func initialize(ctx context.Context, flags flagMap, cfg *appConfig, policiesFile, devicesFile io.ReadCloser) (servicerunner.Runner[appConfig], error) {

	log := logging.GetFromContext(ctx)
	accessObjectAuthz, _ := strconv.ParseBool(flags[authzAccessObject])

	probes := readinessProbes()

	var s *storage.Storage
	var messenger messaging.MsgContext
	var deviceAPI devices.DeviceAPIService
	var sensorAPI sensors.SensorAPIService
	var alarmsAPI alarms.AlarmAPIService
	var wd watchdog.Watchdog
	var deviceStatusHandler devices.DeviceStatusHandler

	var app application.Management

	owned := &ownedResources{}

	_, runner := servicerunner.New(ctx, *cfg,
		webserver("control", listen(flags[listenAddress]), port(flags[controlPort]),
			pprof(), liveness(func() error { return nil }), readiness(probes),
		),
		webserver("public", listen(flags[listenAddress]), port(flags[servicePort]), tracing(tracingEnabled(flags)),
			muxinit(func(ctx context.Context, identifier string, port string, appCfg *appConfig, handler *http.ServeMux) error {
				defer policiesFile.Close()
				return api.RegisterHandlers(ctx, handler, policiesFile, app, auth.WithAccessObjectAuthorization(accessObjectAuthz))
			}),
		),
		oninit(func(ctx context.Context, ac *appConfig) error {
			log.Debug("initializing servicerunner")

			var err error

			s, err = newStorage(ctx, flags)
			if err != nil {
				return fmt.Errorf("could not connect to, or create, database: %w", err)
			}

			messenger, err = initMessenger(ctx, serviceName, log)
			if err != nil {
				s.Close()
				s = nil
				return fmt.Errorf("failed to init messenger: %w", err)
			}

			svc := devices.New(s, s, s, s, messenger, &ac.DeviceManagementConfig)
			deviceAPI = svc
			deviceStatusHandler = svc
			sensorAPI = sensors.New(s, s)
			alarmsAPI = alarms.New(s, messenger, &ac.AlarmServiceConfig)
			wd = watchdog.New(alarmsAPI, &ac.WatchdogConfig)

			owned.watchdog = wd
			owned.messenger = messenger
			owned.tracker = &handlerTracker{}
			owned.storage = s

			app = application.New(deviceAPI, sensorAPI, alarmsAPI, seedExistingDevicesEnabled(flags))

			return nil
		}),
		onstarting(func(ctx context.Context, appCfg *appConfig) (err error) {
			log.Debug("starting servicerunner")

			// OnStarting failures bypass OnShutdown in the runner, so
			// clean up acquired resources on every error path below.
			defer func() {
				if err != nil {
					owned.close(ctx)
				}
			}()

			err = app.SeedLwm2mTypes(ctx, appCfg.DeviceManagementConfig.Types)
			if err != nil {
				return
			}

			err = app.SeedSensorProfiles(ctx, appCfg.DeviceManagementConfig.DeviceProfiles)
			if err != nil {
				return
			}

			err = app.SeedSensorsAndDevices(ctx, devicesFile, strings.Split(flags[allowedSeedTenants], ","))
			if err != nil {
				return
			}

			messenger.Start()

			tracked := &trackingMessenger{MsgContext: messenger, tracker: owned.tracker}

			err = devices.RegisterTopicMessageHandler(ctx, deviceStatusHandler, tracked)
			if err != nil {
				return
			}

			err = alarms.RegisterTopicMessageHandler(ctx, alarmsAPI, tracked)
			if err != nil {
				return
			}

			wd.Start(ctx)

			return nil
		}),
		onshutdown(func(ctx context.Context, appCfg *appConfig) error {
			log.Debug("shutdown servicerunner")

			owned.close(ctx)

			return nil
		}),
	)

	return runner, nil
}

// tracingEnabled is the minimal production seam for the tracing toggle.
// Only the exact string "true" enables tracing; ParseBool spellings
// such as "TRUE" or "1" intentionally do not.
func tracingEnabled(flags flagMap) bool {
	return flags[enableTracing] == "true"
}

// seedExistingDevicesEnabled is the minimal production seam for the
// seed toggle, using strconv semantics: invalid values silently
// disable seeding.
func seedExistingDevicesEnabled(flags flagMap) bool {
	v, _ := strconv.ParseBool(flags[seedExistingDevices])
	return v
}

// readinessProbes returns the named readiness stubs. Per harmonization
// standard they always report OK and never call any dependency.
func readinessProbes() map[string]k8shandlers.ServiceProber {
	return map[string]k8shandlers.ServiceProber{
		"rabbitmq":  func(context.Context) (string, error) { return "ok", nil },
		"timescale": func(context.Context) (string, error) { return "ok", nil },
	}
}

// Shutdown budgets, within the runner's 30s shutdown hook budget. The
// hook itself never receives the runner's timeout, so shutdown derives
// its own bounded contexts here.
const (
	shutdownHandlerDrainTimeout = 10 * time.Second
	shutdownWatchdogTimeout     = 15 * time.Second
)

// handlerTracker tracks admitted topic-message deliveries so shutdown
// can await them. The messaging library acknowledges on dispatch and its
// Close only joins the dispatch loop, never the handler goroutines.
type handlerTracker struct {
	wg sync.WaitGroup
}

func (t *handlerTracker) track(next messaging.TopicMessageHandler) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg messaging.IncomingTopicMessage, log *slog.Logger) {
		t.wg.Add(1)
		defer t.wg.Done()
		next(ctx, msg, log)
	}
}

// wait blocks until tracked handlers complete or the timeout elapses,
// reporting whether all handlers finished.
func (t *handlerTracker) wait(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		defer close(done)
		t.wg.Wait()
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// trackingMessenger decorates handler registration with delivery
// tracking. All other MsgContext behavior is forwarded unchanged.
type trackingMessenger struct {
	messaging.MsgContext
	tracker *handlerTracker
}

func (m *trackingMessenger) RegisterTopicMessageHandler(routingKey string, h messaging.TopicMessageHandler) error {
	return m.MsgContext.RegisterTopicMessageHandler(routingKey, m.tracker.track(h))
}

// ownedResources tracks the resources created during OnInit so shutdown
// is nil-safe, ordered and idempotent. The underlying messenger Close is
// not safe to call twice, hence the sync.Once guard.
//
// Shutdown order: stop inflow (messenger), await admitted handlers
// within budget, stop the watchdog with a real deadline, then close
// storage. HTTP servers stay live until after OnShutdown returns (runner
// behavior); that residual window is documented, not fixed here.
type ownedResources struct {
	once      sync.Once
	watchdog  watchdog.Watchdog
	messenger messaging.MsgContext
	tracker   *handlerTracker
	storage   interface{ Close() }
}

func (o *ownedResources) close(ctx context.Context) {
	o.once.Do(func() {
		if o.messenger != nil {
			o.messenger.Close()
		}
		if o.tracker != nil {
			o.tracker.wait(shutdownHandlerDrainTimeout)
		}
		if o.watchdog != nil {
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownWatchdogTimeout)
			defer cancel()
			o.watchdog.Stop(stopCtx)
		}
		if o.storage != nil {
			o.storage.Close()
		}
	})
}

// initMessenger initializes messaging while converting the library's
// configuration panics (e.g. missing RABBITMQ_HOST) into errors.
func initMessenger(ctx context.Context, serviceName string, log *slog.Logger) (messenger messaging.MsgContext, err error) {
	defer func() {
		if r := recover(); r != nil {
			messenger = nil
			err = fmt.Errorf("messaging initialization panicked: %v", r)
		}
	}()

	messenger, err = messaging.Initialize(ctx, messaging.LoadConfiguration(ctx, serviceName, log))
	return messenger, err
}

func newStorage(ctx context.Context, flags flagMap) (*storage.Storage, error) {
	return storage.New(ctx, storage.NewConfig(flags[dbHost], flags[dbUser], flags[dbPassword], flags[dbPort], flags[dbName], flags[dbSSLMode]))
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

	i := slices.IndexFunc(cfg.DeviceManagementConfig.DeviceProfiles, func(dp types.SensorProfile) bool {
		return dp.Decoder == "unknown"
	})

	if i < 0 {
		cfg.DeviceManagementConfig.DeviceProfiles = append(cfg.DeviceManagementConfig.DeviceProfiles, types.SensorProfile{
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
	flags[authzAccessObject] = envOrDef(ctx, "AUTHZ_ACCESS_OBJECT_ENABLED", flags[authzAccessObject])
	flags[allowedSeedTenants] = envOrDef(ctx, "ALLOWED_SEED_TENANTS", flags[allowedSeedTenants])
	flags[seedExistingDevices] = envOrDef(ctx, "SEED_EXISTING_DEVICES", flags[seedExistingDevices])

	flags[dbHost] = envOrDef(ctx, "POSTGRES_HOST", flags[dbHost])
	flags[dbPort] = envOrDef(ctx, "POSTGRES_PORT", flags[dbPort])
	flags[dbName] = envOrDef(ctx, "POSTGRES_DBNAME", flags[dbName])
	flags[dbUser] = envOrDef(ctx, "POSTGRES_USER", flags[dbUser])
	flags[dbPassword] = envOrDef(ctx, "POSTGRES_PASSWORD", flags[dbPassword])
	flags[dbSSLMode] = envOrDef(ctx, "POSTGRES_SSLMODE", flags[dbSSLMode])

	flags[enableTracing] = envOrDef(ctx, "ENABLE_TRACING", flags[enableTracing])

	flags[logLevel] = envOrDef(ctx, "LOG_LEVEL", flags[logLevel])

	apply := func(f flagType) func(string) error {
		return func(value string) error {
			flags[f] = value
			return nil
		}
	}

	// Allow command line arguments to override defaults and environment variables
	flag.Func("policies", "an authorization policy file", apply(policiesFile))
	flag.Func("authz-access-object", "enable access-object authorization policy result model", apply(authzAccessObject))
	flag.Func("devices", "list of known devices", apply(devicesFile))
	flag.Func("config", "device management configuration file", apply(configurationFile))
	flag.Func("devmode", "enable dev mode", apply(devmode))
	flag.Func("loglevel", "set the log level", apply(logLevel))
	flag.Parse()

	return ctx, flags
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelDebug
	}
}

func exitIf(err error, logger *slog.Logger, msg string, args ...any) {
	if err != nil {
		logger.With(args...).Error(msg, "err", err.Error())
		time.Sleep(2 * time.Second)
		os.Exit(1)
	}
}
