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
	db "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	aDb "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/alarms"
	dmDb "github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database/devicemanagement"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y/logging"
	"github.com/go-chi/chi/v5"
)

const serviceName string = "iot-device-mgmt"

var knownDevicesFile string
var opaFilePath string
var alarmConfigFile string

func main() {
	serviceVersion := buildinfo.SourceVersion()
	ctx, _, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&knownDevicesFile, "devices", "/opt/diwise/config/devices.csv", "A file containing known devices")
	flag.StringVar(&alarmConfigFile, "alarms", "/opt/diwise/config/alarms.csv", "A file containing alarms")
	flag.StringVar(&opaFilePath, "policies", "/opt/diwise/config/authz.rego", "An authorization policy file")
	flag.Parse()

	conn := setupDatabaseConnection(ctx)
	deviceDB := setupDeviceDatabaseOrDie(ctx, conn)
	alarmDB := setupAlarmDatabaseOrDie(ctx, conn)

	messenger := setupMessagingOrDie(ctx, serviceName)

	mgmtSvc := devicemanagement.New(deviceDB, messenger)

	alarmSvc := alarms.New(alarmDB, messenger, alarms.LoadConfiguration(alarmConfigFile))
	alarmSvc.Start()
	defer alarmSvc.Stop()

	watchdog := watchdog.New(deviceDB, messenger)
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

func setupDatabaseConnection(ctx context.Context) db.ConnectorFunc {
	if os.Getenv("POSTGRES_HOST") != "" {
		return db.NewPostgreSQLConnector(ctx, db.LoadConfigFromEnv(ctx))
	}

	logger := logging.GetFromContext(ctx)
	logger.Info("no sql database configured, using builtin sqlite instead")
	return db.NewSQLiteConnector(ctx)
}

func setupAlarmDatabaseOrDie(ctx context.Context, conn db.ConnectorFunc) aDb.AlarmRepository {
	var db aDb.AlarmRepository
	var err error

	db, err = aDb.NewAlarmRepository(conn)
	if err != nil {
		fatal(ctx, "failed to connect to database", err)
	}

	return db
}

func setupDeviceDatabaseOrDie(ctx context.Context, conn db.ConnectorFunc) dmDb.DeviceRepository {
	var db dmDb.DeviceRepository
	var err error

	db, err = dmDb.NewDeviceRepository(conn)
	if err != nil {
		fatal(ctx, "failed to connect to database", err)
	}

	if _, err := os.Stat(knownDevicesFile); os.IsNotExist(err) {
		fatal(ctx, fmt.Sprintf("file with known devices (%s) could not be found", knownDevicesFile), err)
	}

	f, err := os.Open(knownDevicesFile)
	if err != nil {
		fatal(ctx, fmt.Sprintf("file with known devices (%s) could not be opened", knownDevicesFile), err)
	}
	defer f.Close()

	err = db.Seed(ctx, f)
	if err != nil {
		fatal(ctx, "could not seed database with devices", err)
	}

	return db
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
