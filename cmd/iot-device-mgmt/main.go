package main

import (
	"context"
	"net/http"
	"runtime/debug"

	"github.com/diwise/iot-device-mgmt/internal/pkg/application"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/logging"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/router"
	"github.com/diwise/iot-device-mgmt/internal/pkg/infrastructure/tracing"
	"github.com/diwise/iot-device-mgmt/internal/pkg/presentation/api"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

const serviceName string = "iot-device-mgmt"

func main() {
	serviceVersion := version()

	ctx, logger := logging.NewLogger(context.Background(), serviceName, serviceVersion)
	logger.Info().Msg("starting up ...")

	cleanup, err := tracing.Init(ctx, logger, serviceName, serviceVersion)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to init tracing")
	}
	defer cleanup()

	r := createAppAndSetupRouter(logger, serviceName)

	err = http.ListenAndServe(":8080", r)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to start router")
	}
}

func createAppAndSetupRouter(logger zerolog.Logger, serviceName string) *chi.Mux {
	app := application.New()
	r := router.New(serviceName)
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
