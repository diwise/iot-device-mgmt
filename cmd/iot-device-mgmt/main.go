package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application/alarms"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/service"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
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

func main() {
	serviceVersion := buildinfo.SourceVersion()
	_, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&knownDevicesFile, "devices", "/opt/diwise/config/devices.csv", "A file containing known devices")
	flag.StringVar(&opaFilePath, "policies", "/opt/diwise/config/authz.rego", "An authorization policy file")
	flag.Parse()

	apiPort := fmt.Sprintf(":%s", env.GetVariableOrDefault(logger, "SERVICE_PORT", "8080"))

	conn := setupDatabaseConnection(logger)

	deviceDB := setupDeviceDatabaseOrDie(logger, conn)
	alarmDB := setupAlarmDatabaseOrDie(logger, conn)
	messenger := setupMessagingOrDie(serviceName, logger)

	service := service.New(deviceDB, messenger)
	alarmService := alarms.New(alarmDB, messenger)

	watchdog := watchdog.New(deviceDB, messenger, logger)
	watchdog.Start()
	defer watchdog.Stop()

	r := setupRouter(logger, serviceName, service, alarmService)

	err := http.ListenAndServe(apiPort, r)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start router")
	}
}

func setupDatabaseConnection(logger zerolog.Logger) database.ConnectorFunc {
	if os.Getenv("DIWISE_SQLDB_HOST") != "" {
		return database.NewPostgreSQLConnector(logger)
	}

	logger.Info().Msg("no sql database configured, using builtin sqlite instead")
	return database.NewSQLiteConnector(logger)
}

func setupAlarmDatabaseOrDie(logger zerolog.Logger, conn database.ConnectorFunc) database.AlarmRepository {
	var db database.AlarmRepository
	var err error

	db, err = database.NewAlarmRepository(conn)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}

	return db
}

func setupDeviceDatabaseOrDie(logger zerolog.Logger, conn database.ConnectorFunc) database.DeviceRepository {
	var db database.DeviceRepository
	var err error

	db, err = database.NewDeviceRepository(conn)
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

	err = db.Seed(context.Background(), knownDevicesFile, f)
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

func setupRouter(logger zerolog.Logger, serviceName string, svc service.DeviceManagement, alarmSvc alarms.AlarmService) *chi.Mux {
	r := router.New(serviceName)

	policies, err := os.Open(opaFilePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to open opa policy file")
	}
	defer policies.Close()

	return api.RegisterHandlers(logger, r, policies, svc, alarmSvc)
}
