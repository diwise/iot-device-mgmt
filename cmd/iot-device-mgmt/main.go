package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/storage"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v2"
)

const serviceName string = "iot-device-mgmt"

var (
	knownDevicesFile  string
	opaFilePath       string
	configurationFile string
)

func main() {
	serviceVersion := buildinfo.SourceVersion()
	ctx, _, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&knownDevicesFile, "devices", "/opt/diwise/config/devices.csv", "A file containing known devices")
	flag.StringVar(&configurationFile, "config", "/opt/diwise/config/config.yaml", "A yaml file containing configuration data")
	flag.StringVar(&opaFilePath, "policies", "/opt/diwise/config/authz.rego", "An authorization policy file")
	flag.Parse()

	storage := setupStorageOrDie(ctx)

	messenger := setupMessagingOrDie(ctx, serviceName)
	messenger.Start()

	mgmtSvc := devicemanagement.New(storage, messenger, loadConfigurationOrDie(ctx))
	alarmSvc := alarms.New(storage, messenger)

	seedDataOrDie(ctx, mgmtSvc)

	watchdog := watchdog.New(mgmtSvc, messenger)
	watchdog.Start(ctx)
	defer watchdog.Stop(ctx)

	r, err := setupRouter(ctx, serviceName, mgmtSvc, alarmSvc)
	if err != nil {
		fatal(ctx, "failed to setup router", err)
	}

	apiPort := fmt.Sprintf(":%s", env.GetVariableOrDefault(ctx, "SERVICE_PORT", "8080"))

	err = http.ListenAndServe(apiPort, r)
	if err != nil {
		fatal(ctx, "failed to start router", err)
	}
}

func setupStorageOrDie(ctx context.Context) *storage.Storage {
	var err error

	p, err := storage.NewPool(ctx, storage.LoadConfiguration(ctx))
	if err != nil {
		panic(err)
	}

	s := storage.NewWithPool(p)
	err = s.CreateTables(ctx)
	if err != nil {
		panic(err)
	}

	return s
}

func loadConfigurationOrDie(ctx context.Context) *devicemanagement.DeviceManagementConfig {
	if _, err := os.Stat(configurationFile); os.IsNotExist(err) {
		fatal(ctx, "configuration file not found", err)
	}

	f, err := os.Open(configurationFile)
	if err != nil {
		fatal(ctx, "could not open configuration", err)
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		fatal(ctx, "could not read configuration file", err)
	}

	cfg := &devicemanagement.DeviceManagementConfig{}
	err = yaml.Unmarshal(b, cfg)
	if err != nil {
		fatal(ctx, "could not unmarshal configuration file", err)
	}

	return cfg
}

func seedDataOrDie(ctx context.Context, svc devicemanagement.DeviceManagement) {
	if _, err := os.Stat(knownDevicesFile); os.IsNotExist(err) {
		fatal(ctx, fmt.Sprintf("file with known devices (%s) could not be found", knownDevicesFile), err)
	}

	f, err := os.Open(knownDevicesFile)
	if err != nil {
		fatal(ctx, fmt.Sprintf("file with known devices (%s) could not be opened", knownDevicesFile), err)
	}
	defer f.Close()

	logger := logging.GetFromContext(ctx)

	tenants := env.GetVariableOrDefault(ctx, "ALLOWED_SEED_TENANTS", "default")

	logger.Debug(fmt.Sprintf("Allowed seed tenants: %s", tenants))

	err = svc.Seed(ctx, f, strings.Split(tenants, ","))
	if err != nil {
		fatal(ctx, "could not seed database with devices", err)
	}
}

func setupMessagingOrDie(ctx context.Context, serviceName string) messaging.MsgContext {
	logger := logging.GetFromContext(ctx)

	config := messaging.LoadConfiguration(ctx, serviceName, logger)
	messenger, err := messaging.Initialize(ctx, config)
	if err != nil {
		fatal(ctx, "failed to init messenger", err)
	}

	return messenger
}

func setupRouter(ctx context.Context, serviceName string, svc devicemanagement.DeviceManagement, alarmSvc alarms.AlarmService) (*chi.Mux, error) {
	r := router.New(serviceName)

	policies, err := os.Open(opaFilePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open opa policy file: %s", err.Error())
	}
	defer policies.Close()

	return api.RegisterHandlers(ctx, r, policies, svc, alarmSvc)
}

func fatal(ctx context.Context, msg string, err error) {
	logger := logging.GetFromContext(ctx)
	logger.Error(msg, "err", err.Error())
	os.Exit(1)
}
