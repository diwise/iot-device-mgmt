package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog"
	alarmStore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/alarms"
	deviceStore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/devicemanagement"
	jsonstore "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/jsonstorage"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const serviceName string = "iot-device-mgmt"

var knownDevicesFile string
var opaFilePath string

func main() {
	serviceVersion := buildinfo.SourceVersion()
	ctx, _, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&knownDevicesFile, "devices", "/opt/diwise/data/devices.csv", "A file containing known devices")
	flag.StringVar(&opaFilePath, "policies", "/opt/diwise/config/authz.rego", "An authorization policy file")
	flag.Parse()

	p, err := jsonstore.NewPool(ctx, jsonstore.LoadConfiguration(ctx))
	if err != nil {
		panic(err)
	}

	deviceStorage := setupDeviceDatabaseOrDie(ctx, p)
	alarmStorage := setupAlarmDatabaseOrDie(ctx, p)

	messenger := setupMessagingOrDie(ctx, serviceName)
	messenger.Start()

	mgmtSvc := devicemanagement.New(deviceStorage, messenger)
	alarmSvc := alarms.New(alarmStorage, messenger)

	watchdog := watchdog.New(deviceStorage, messenger)
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

func setupAlarmDatabaseOrDie(ctx context.Context, p *pgxpool.Pool) alarmStore.AlarmRepository {
	repo, err := alarmStore.NewRepository(ctx, p)
	if err != nil {
		panic(err)
	}
	return repo
}

func setupDeviceDatabaseOrDie(ctx context.Context, p *pgxpool.Pool) deviceStore.DeviceRepository {
	var err error

	repo, err := deviceStore.NewRepository(ctx, p)
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(knownDevicesFile); os.IsNotExist(err) {
		fatal(ctx, fmt.Sprintf("file with known devices (%s) could not be found", knownDevicesFile), err)
	}

	f, err := os.Open(knownDevicesFile)
	if err != nil {
		fatal(ctx, fmt.Sprintf("file with known devices (%s) could not be opened", knownDevicesFile), err)
	}
	defer f.Close()

	err = repo.Seed(ctx, f, []string{"default"})
	if err != nil {
		fatal(ctx, "could not seed database with devices", err)
	}

	return repo
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
