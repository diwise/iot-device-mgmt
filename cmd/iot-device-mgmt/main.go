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
	"path"
	"path/filepath"
	"sort"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/events"
	"github.com/diwise/iot-device-mgmt/internal/pkg/application/watchdog"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/iot-device-mgmt/pkg/types"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/diwise/service-chassis/pkg/infrastructure/buildinfo"
	"github.com/diwise/service-chassis/pkg/infrastructure/env"
	"github.com/diwise/service-chassis/pkg/infrastructure/o11y"
	"github.com/go-chi/chi/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

const serviceName string = "iot-device-mgmt"

var dataDir string
var opaFilePath string
var notificationConfigPath string

func main() {
	serviceVersion := buildinfo.SourceVersion()
	_, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&dataDir, "devices", "/opt/diwise/config/data", "A directory containing data of known devices (devices.csv) & sensorTypes (sensorTypes.csv)")
	flag.StringVar(&opaFilePath, "policies", "/opt/diwise/config/authz.rego", "An authorization policy file")
	flag.StringVar(&notificationConfigPath, "notifications", "/opt/diwise/config/notifications.yaml", "Configuration file for notifications")
	flag.Parse()

	apiPort := fmt.Sprintf(":%s", env.GetVariableOrDefault(logger, "SERVICE_PORT", "8080"))

	db := setupDatabaseOrDie(logger)
	messenger := setupMessagingOrDie(serviceName, logger)
	eventSender := events.New(loadEventSenderConfig(logger))

	app := application.New(db, eventSender, messenger)

	routingKey := "device-status"
	messenger.RegisterTopicMessageHandler(routingKey, newDeviceTopicMessageHandler(messenger, app))

	watchdog := watchdog.New(app, logger)
	watchdog.Start()
	defer watchdog.Stop()

	r := setupRouter(logger, serviceName, app)

	err := http.ListenAndServe(apiPort, r)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start router")
	}
}

func setupDatabaseOrDie(logger zerolog.Logger) database.Datastore {
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

	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		logger.Fatal().Err(err).Msgf("directory %s does not exists! Unable to load data", dataDir)
	}

	files, err := filepath.Glob(dataDir + "/*.csv")
	if err != nil {
		logger.Fatal().Err(err).Msg("no data files found!")
	}

	logger.Debug().Msgf("found %d files in %s", len(files), dataDir)

	sort.Strings(files)

	for _, f := range files {
		dataFile, err := os.Open(f)
		if err == nil {
			defer dataFile.Close()
			logger.Debug().Msgf("Seeding %s", path.Base(dataFile.Name()))
			err = db.Seed(path.Base(dataFile.Name()), dataFile)
			if err != nil {
				logger.Fatal().Err(err).Msg("failed to seed database")
			}
		}
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

func loadEventSenderConfig(logger zerolog.Logger) *events.Config {
	if nCfgFile, err := os.Open(notificationConfigPath); err == nil {
		defer nCfgFile.Close()

		nCfg, err := events.LoadConfiguration(nCfgFile)
		if err != nil {
			logger.Fatal().Err(err).Msg("failed to load configuration")
		}

		return nCfg
	} else if !errors.Is(err, fs.ErrNotExist) {
		logger.Fatal().Err(err).Msgf("failed to open configuration file %s", notificationConfigPath)
	}
	return nil
}

func setupRouter(logger zerolog.Logger, serviceName string, app application.App) *chi.Mux {
	r := router.New(serviceName)

	policies, err := os.Open(opaFilePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to open opa policy file")
	}
	defer policies.Close()

	return api.RegisterHandlers(logger, r, policies, app)
}

func newDeviceTopicMessageHandler(messenger messaging.MsgContext, app application.App) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {
		logger.Debug().Str("body", string(msg.Body)).Msg("received message")

		ds := types.DeviceStatus{}

		err := json.Unmarshal(msg.Body, &ds)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal body of accepted message")
			return
		}

		err = app.HandleDeviceStatus(ctx, ds)
		if err != nil {
			logger.Error().Err(err).Msg("failed to handle device status message")
			return
		}

		logger.Info().Msg("message handled")
	}
}
