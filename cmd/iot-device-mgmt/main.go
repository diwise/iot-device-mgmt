package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/go-chi/chi/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

const serviceName string = "iot-device-mgmt"

var devicesFilePath string
var opaFilePath string
var notificationConfigPath string

func main() {
	serviceVersion := buildinfo.SourceVersion()
	_, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&devicesFilePath, "devices", "/opt/diwise/config/devices.csv", "A file of known devices")
	flag.StringVar(&opaFilePath, "policies", "/opt/diwise/config/authz.rego", "An authorization policy file")
	flag.StringVar(&notificationConfigPath, "notifications", "/opt/diwise/config/notifications.yaml", "Configuration file for notifications")
	flag.Parse()

	db := connectToDatabaseOrDie(logger)

	devicesFile, err := os.Open(devicesFilePath)
	if err == nil {
		defer devicesFile.Close()

		logger.Info().Msgf("seeding database from %s", devicesFilePath)
		err = db.Seed(devicesFile)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to seed database")
		}
	}

	config := messaging.LoadConfiguration(serviceName, logger)
	messenger, err := messaging.Initialize(config)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to init messenger")
	}

	nCfg := &application.Config{}
	if nCfgFile, err := os.Open(notificationConfigPath); err == nil {
		defer nCfgFile.Close()
		
		nCfg, err = application.LoadConfiguration(nCfgFile)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to load configuration")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		logger.Fatal().Err(err).Msg("failed to open file")
	}

	r := createAppAndSetupRouter(logger, serviceName, db, messenger, nCfg)

	apiPort := fmt.Sprintf(":%s", env.GetVariableOrDefault(logger, "SERVICE_PORT", "8080"))

	err = http.ListenAndServe(apiPort, r)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start router")
	}
}

func connectToDatabaseOrDie(logger zerolog.Logger) database.Datastore {
	var db database.Datastore
	var err error

	if os.Getenv("DIWISE_SQLDB_HOST") != "" {
		db, err = database.NewDatabaseConnection(database.NewPostgreSQLConnector(logger))
	} else {
		logger.Info().Msg("no sql database configured, using builtin sqlite instead")
		db, err = database.NewDatabaseConnection(database.NewSQLiteConnector(logger))
	}

	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}

	return db
}

func createAppAndSetupRouter(logger zerolog.Logger, serviceName string, db database.Datastore, messenger messaging.MsgContext, cfg *application.Config) *chi.Mux {
	app := application.New(db, cfg)

	routingKey := "device-status"
	messenger.RegisterTopicMessageHandler(routingKey, newTopicMessageHandler(messenger, app))

	r := router.New(serviceName)

	policies, err := os.Open(opaFilePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to open opa policy file")
	}
	defer policies.Close()

	return api.RegisterHandlers(logger, r, policies, app)
}

func newTopicMessageHandler(messenger messaging.MsgContext, app application.DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger.Info().Str("body", string(msg.Body)).Msg("received message")

		statusMessage := application.StatusMessage{}

		err := json.Unmarshal(msg.Body, &statusMessage)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal body of accepted message")
			return
		}

		timestamp, err := time.Parse(time.RFC3339, statusMessage.Timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("failed to parse time from status message")
			return
		}

		err = app.UpdateLastObservedOnDevice(statusMessage.DeviceID, timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("failed to handle accepted message")
			return
		}

		err = app.NotifyStatus(ctx, statusMessage)
		if err != nil {
			logger.Error().Err(err).Msg("failed to send notification")
			return
		}
	}
}
