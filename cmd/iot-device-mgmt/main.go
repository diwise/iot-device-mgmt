package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

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

	db := setupDatabaseOrDie(logger)
	messenger := setupMessagingOrDie(serviceName, logger)

	service := service.New(db, messenger)

	watchdog := watchdog.New(db, messenger, logger)
	watchdog.Start()
	defer watchdog.Stop()

	r := setupRouter(logger, serviceName, service)

	err := http.ListenAndServe(apiPort, r)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start router")
	}
}

func setupDatabaseOrDie(logger zerolog.Logger) database.DeviceRepository {
	var db database.DeviceRepository
	var err error

	if os.Getenv("DIWISE_SQLDB_HOST") != "" {
		db, err = database.New(database.NewPostgreSQLConnector(logger))
	} else {
		logger.Info().Msg("no sql database configured, using builtin sqlite instead")
		db, err = database.New(database.NewSQLiteConnector(logger))
	}

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

func setupRouter(logger zerolog.Logger, serviceName string, service service.DeviceManagement) *chi.Mux {
	r := router.New(serviceName)

	policies, err := os.Open(opaFilePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to open opa policy file")
	}
	defer policies.Close()

	return api.RegisterHandlers(logger, r, policies, service)
}

