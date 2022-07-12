package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
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

func main() {
	serviceVersion := buildinfo.SourceVersion()
	_, logger, cleanup := o11y.Init(context.Background(), serviceName, serviceVersion)
	defer cleanup()

	flag.StringVar(&devicesFilePath, "devices", "/opt/diwise/config/devices.csv", "A file of known devices")
	flag.Parse()

	dsn := env.GetVariableOrDie(logger, "POSTGRE_DSN", "DSN for postgre database")

	db, err := database.ConnectDb(dsn)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to database")
	}

	db.Seed(devicesFilePath)

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

		err = app.UpdateLastObservedOnDevice(statusMessage.DeviceID, timestamp)
		if err != nil {
			logger.Error().Err(err).Msg("failed to handle accepted message")
		}
	}
}

func createAppAndSetupRouter(logger zerolog.Logger, serviceName string, db database.Datastore, messenger messaging.MsgContext) *chi.Mux {
	app := application.New(db)

	routingKey := "device-status"
	messenger.RegisterTopicMessageHandler(routingKey, newTopicMessageHandler(messenger, app))

	r := router.New(serviceName)

	return api.RegisterHandlers(logger, r, app)
}
