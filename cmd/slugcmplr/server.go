package main

import (
	"context"
	"fmt"
	"os"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/store"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

func serverCmd(verbose bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a slugmcplr server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputterFromCmd(cmd, verbose)

			if err := defaultEnv(); err != nil {
				return fmt.Errorf("failed to set env defaults: %w", err)
			}

			env, err := requireEnv(
				"PORT",
				"SLUGCMPLR_ENV",
				"DATABASE_URL",
			)
			if err != nil {
				return fmt.Errorf("error fetching environment: %w", err)
			}

			closer, err := initObs(cmd, output, env)
			if err != nil {
				return err
			}
			defer closer()

			// Handle this error in a sensible manner where possible
			querier, err := store.Build(env["DATABASE_URL"])
			if err != nil {
				return err
			}

			s := &slugcmplr.ServerCmd{
				Port:        env["PORT"],
				Environment: env["SLUGCMPLR_ENV"],
				Store:       querier,
			}

			return s.Execute(cmd.Context(), output)
		},
	}

	return cmd
}

func defaultEnv() error {
	defaults := map[string]string{
		"SLUGCMPLR_ENV": "development",
		"PORT":          "1123",
		"DATABASE_URL":  "postgres://localhost:5432/slugcmplr_development?sslmode=disable",
	}
	for k, v := range defaults {
		if _, ok := os.LookupEnv(k); ok {
			continue
		}

		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}

	return nil
}

func otelExporter(ctx context.Context, env map[string]string) (sdktrace.SpanExporter, error) {
	if env["SLUGCMPLR_ENV"] == "production" {
		// OTEL OTLP exporters can be configured with the following ENV vars:
		// - OTEL_EXPORTER_OTLP_ENDPOINT (e.g. https://api.honeycomb.io:443)
		// - OTEL_EXPORTER_OTLP_HEADERS (e.g. x-honeycomb-team=<API-KEY>,x-honeycomb-dataset=<dataset>)
		// - OTEL_EXPORTER_OTLP_COMPRESSION (e.g. gzip)
		// - OTEL_EXPORTER_OTLP_PROTOCOL (e.g. grpc)
		// - OTEL_EXPORTER_OTLP_CERTIFICATE (e.g. /etc/ssl/certs/ca-certificates.crt)
		return otlptracegrpc.New(ctx)
	}

	return stdouttrace.New()
}

func initObs(cmd *cobra.Command, output slugcmplr.Outputter, env map[string]string) (func(), error) {
	exp, err := otelExporter(cmd.Context(), env)
	if err != nil {
		return nil, fmt.Errorf("failed to setup otelgrpc exporter: %w", err)
	}
	bsp := sdktrace.NewBatchSpanProcessor(exp)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(
			// Default attributes for every span.
			//
			// The following are provided by runtime-dyno-metadata feature in Heroku
			// See: https://devcenter.heroku.com/articles/dyno-metadata
			// - HEROKU_APP_NAME
			// - HEROKU_DYNO_ID
			// - HEROKU_RELEASE_VERSION
			// - HEROKU_SLUG_COMMIT
			//
			// The Heroku runtime provides the following environment variables:
			// See: https://devcenter.heroku.com/articles/dynos#local-environment-variables
			// - DYNO
			//
			// These environment variables are expected to be managed
			// by the application owner:
			// - SLUGCMPLR_HEROKU_ACCOUNT
			// - SLUGCMPLR_HEROKU_STACK
			// - SLUGCMPLR_ENV
			resource.NewWithAttributes(
				"https://opentelemetry.io/schemas/v1.4.0",

				// Cloud keys
				semconv.CloudProviderKey.String("heroku"),
				semconv.CloudAccountIDKey.String(os.Getenv("SLUGCMPLR_HEROKU_ACCOUNT")),
				semconv.CloudPlatformKey.String(os.Getenv("SLUGCMPLR_HEROKU_STACK")),

				// Service keys
				semconv.ServiceNamespaceKey.String(os.Getenv("HEROKU_APP_NAME")),
				semconv.ServiceNameKey.String("slugcmplr-http"),
				semconv.ServiceInstanceIDKey.String(os.Getenv("HEROKU_DYNO_ID")),
				semconv.ServiceVersionKey.String(os.Getenv("HEROKU_RELEASE_VERSION")),
				attribute.String("service.revision", os.Getenv("HEROKU_SLUG_COMMIT")),

				// Deplyment environment
				semconv.DeploymentEnvironmentKey.String(os.Getenv("SLUGCMPLR_ENV")),
			),
		),
	)

	otel.SetTracerProvider(tp)
	propagator := propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{})
	otel.SetTextMapPropagator(propagator)

	return func() {
		if err := tp.Shutdown(cmd.Context()); err != nil {
			fmt.Fprintf(output.ErrOrStderr(), "error shutting down tracing: %v", err)
		}
	}, nil
}
