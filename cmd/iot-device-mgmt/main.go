package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/logging"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/repositories/database"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/tracing"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/gui"
	"github.com/diwise/messaging-golang/pkg/messaging"
	"github.com/go-chi/chi/v5"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog"
)

const serviceName string = "iot-device-mgmt"

var devicesFilePath string

func main() {
	serviceVersion := version()

	ctx, logger := logging.NewLogger(context.Background(), serviceName, serviceVersion)
	logger.Info().Msg("starting up ...")

	flag.StringVar(&devicesFilePath, "devices", "/opt/diwise/config/devices.csv", "A file of known devices")
	flag.Parse()

	cleanup, err := tracing.Init(ctx, logger, serviceName, serviceVersion)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to init tracing")
	}
	defer cleanup()

	db, err := setupDatabase(logger, devicesFilePath)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start database")
	}

	config := messaging.LoadConfiguration(serviceName, logger)
	messenger, err := messaging.Initialize(config)

	if err != nil {
		logger.Fatal().Err(err).Msg("failed to init messenger")
	}

	r := createAppAndSetupRouter(logger, serviceName, db, messenger)

	err = http.ListenAndServe(":8080", r)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start router")
	}
}

func newTopicMessageHandler(messenger messaging.MsgContext, app application.DeviceManagement) messaging.TopicMessageHandler {
	return func(ctx context.Context, msg amqp.Delivery, logger zerolog.Logger) {

		logger.Info().Str("body", string(msg.Body)).Msg("received message")

		statusMessage := struct {
			DeviceID  string `json:"deviceID"`
			Timestamp string `json:"timestamp"`
		}{}

		err := json.Unmarshal(msg.Body, &statusMessage)
		if err != nil {
			logger.Error().Err(err).Msg("failed to unmarshal body of accepted message")
		}

		timestamp, err := time.Parse(time.RFC3339, statusMessage.Timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("failed to parse time from status message")
		}

		_, err = app.UpdateLastObservedOnDevice(statusMessage.DeviceID, timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("failed to handle accepted message")
		}
	}
}

func setupDatabase(logger zerolog.Logger, filePath string) (database.Datastore, error) {
	devicesFile, err := os.Open(filePath)
	if err != nil {
		logger.Fatal().Err(err).Msgf("failed to open the file of known devices %s", filePath)
	}

	defer devicesFile.Close()

	db, err := database.SetUpNewDatabase(logger, devicesFile)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func createAppAndSetupRouter(logger zerolog.Logger, serviceName string, db database.Datastore, messenger messaging.MsgContext) *chi.Mux {
	app := application.New(db)

	routingKey := "device-status"
	messenger.RegisterTopicMessageHandler(routingKey, newTopicMessageHandler(messenger, app))

	r := router.New(serviceName)

	r = gui.RegisterHandlers(logger, r, app)

	return api.RegisterHandlers(logger, r, app)
}

func version() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	buildSettings := buildInfo.Settings
	infoMap := map[string]string{}
	for _, s := range buildSettings {
		infoMap[s.Key] = s.Value
	}

	sha := infoMap["vcs.revision"]
	if infoMap["vcs.modified"] == "true" {
		sha += "+"
	}

	return sha
}
