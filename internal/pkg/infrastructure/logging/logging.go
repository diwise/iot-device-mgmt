package logging

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type loggerContextKey struct {
	name string
}

var loggerCtxKey = &loggerContextKey{"logger"}

func NewLogger(ctx context.Context, serviceName, serviceVersion string) (context.Context, zerolog.Logger) {
	logger := log.With().Str("service", strings.ToLower(serviceName)).Str("version", serviceVersion).Logger()
	ctx = NewContextWithLogger(ctx, logger)
	return ctx, logger
}

func NewContextWithLogger(ctx context.Context, logger zerolog.Logger) context.Context {
	ctx = context.WithValue(ctx, loggerCtxKey, logger)
	return ctx
}

func GetLoggerFromContext(ctx context.Context) zerolog.Logger {
	logger, ok := ctx.Value(loggerCtxKey).(zerolog.Logger)

	if !ok {
		return log.Logger
	}

	return logger
}
