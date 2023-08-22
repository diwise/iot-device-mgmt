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
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

const serviceName string = "iot-device-mgmt"

var knownDevicesFile string
var opaFilePath string
var alarmConfigFile string

func main() {
	serviceVersion := buildinfo.SourceVersion()
	ctx, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&knownDevicesFile, "devices", "/opt/diwise/config/devices.csv", "A file containing known devices")
	flag.StringVar(&alarmConfigFile, "alarms", "/opt/diwise/config/alarms.csv", "A file containing alarms")
	flag.StringVar(&opaFilePath, "policies", "/opt/diwise/config/authz.rego", "An authorization policy file")
	flag.Parse()

	conn := setupDatabaseConnection(logger)
	deviceDB := setupDeviceDatabaseOrDie(logger, conn)
	alarmDB := setupAlarmDatabaseOrDie(logger, conn)

	messenger := setupMessagingOrDie(serviceName, logger)

	mgmtSvc := devicemanagement.New(deviceDB, messenger)

	alarmSvc := alarms.New(alarmDB, messenger, alarms.LoadConfiguration(alarmConfigFile))
	alarmSvc.Start()
	defer alarmSvc.Stop()

	watchdog := watchdog.New(deviceDB, messenger, logger)
	watchdog.Start()
	defer watchdog.Stop()

	r, err := setupRouter(ctx, serviceName, mgmtSvc, alarmSvc)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to setup router")
	}

	apiPort := fmt.Sprintf(":%s", env.GetVariableOrDefault(logger, "SERVICE_PORT", "8080"))

	err = http.ListenAndServe(apiPort, r)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start router")
	}
}

func setupDatabaseConnection(logger zerolog.Logger) db.ConnectorFunc {
	if os.Getenv("DIWISE_SQLDB_HOST") != "" {
		return db.NewPostgreSQLConnector(logger, db.LoadConfigFromEnv(logger))
	}

	logger.Info().Msg("no sql database configured, using builtin sqlite instead")
	return db.NewSQLiteConnector(logger)
}

func setupAlarmDatabaseOrDie(logger zerolog.Logger, conn db.ConnectorFunc) aDb.AlarmRepository {
	var db aDb.AlarmRepository
	var err error

	db, err = aDb.NewAlarmRepository(conn)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}

	return db
}

func setupDeviceDatabaseOrDie(logger zerolog.Logger, conn db.ConnectorFunc) dmDb.DeviceRepository {
	var db dmDb.DeviceRepository
	var err error

	db, err = dmDb.NewDeviceRepository(conn)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}

	if _, err := os.Stat(knownDevicesFile); os.IsNotExist(err) {
		logger.Fatal().Err(err).Msgf("file with known devices (%s) could not be found", knownDevicesFile)
	}

	f, err := os.Open(knownDevicesFile)
	if err != nil {
		logger.Fatal().Err(err).Msgf("file with known devices (%s) could not be opened", knownDevicesFile)
	}
	defer f.Close()

	err = db.Seed(context.Background(), f)
	if err != nil {
		logger.Fatal().Err(err).Msg("could not seed database with devices")
	}

	return db
}

func setupMessagingOrDie(serviceName string, logger zerolog.Logger) messaging.MsgContext {
	config := messaging.LoadConfiguration(serviceName, logger)
	messenger, err := messaging.Initialize(config)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to init messenger")
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

	return api.RegisterHandlers(ctx, r, policies, svc, alarmSvc), nil
}
