package main

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpgrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv"
	"google.golang.org/grpc/credentials"
)

// InitObs sets up opentelemetry tracing with Honeycomb.
func InitObs(ctx context.Context, key, dataset, svc string) (func(context.Context) error, error) {
	exp, err := otlp.NewExporter(
		ctx,
		otlpgrpc.NewDriver(
			otlpgrpc.WithEndpoint("api.honeycomb.io:443"),
			otlpgrpc.WithHeaders(map[string]string{
				"x-honeycomb-team":    key,
				"x-honeycomb-dataset": dataset,
			}),
			otlpgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, "")),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.ServiceNameKey.String(svc),
				attribute.String("heroku.dyno", os.Getenv("DYNO")),
				attribute.String("heroku.dyno_id", os.Getenv("HEROKU_DYNO_ID")),
				attribute.String("heroku.app_name", os.Getenv("HEROKU_APP_NAME")),
				attribute.String("heroku.app_id", os.Getenv("HEROKU_APP_ID")),
				attribute.String("heroku.release_version", os.Getenv("HEROKU_RELEASE_VERSION")),
				attribute.String("heroku.slug_commit", os.Getenv("HEROKU_SLUG_COMMIT")),
			),
		),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	)

	return tp.Shutdown, nil
}
