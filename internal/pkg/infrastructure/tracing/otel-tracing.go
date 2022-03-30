package tracing

import (
	"context"
	"os"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
)

type CleanupFunc func()

func Init(ctx context.Context, logger zerolog.Logger, serviceName, serviceVersion string) (CleanupFunc, error) {

	exporterEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	cleanupFunc := func() {}

	if exporterEndpoint != "" {
		client := otlptracehttp.NewClient()
		exporter, err := otlptrace.New(ctx, client)
		if err != nil {
			logger.Fatal().Msgf("creating OTLP trace exporter: %v", err)
		}

		tracerProvider := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(newResource(serviceName, serviceVersion)),
		)
		otel.SetTracerProvider(tracerProvider)

		cleanupFunc = func() {
			if err := tracerProvider.Shutdown(ctx); err != nil {
				logger.Fatal().Msgf("stopping tracer provider: %v", err)
			}
		}
	}

	return cleanupFunc, nil
}

// newResource returns a resource describing this application.
func newResource(serviceName, version string) *resource.Resource {
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(version),
	)
}
