package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/bgentry/go-netrc/netrc"
	"github.com/cga1123/slugcmplr"
	heroku "github.com/heroku/heroku-go/v5"
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

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd := Cmd()
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

// Cmd configures the entrypoint to the slugcmplr CLI with all its subcommands.
func Cmd() *cobra.Command {
	var verbose bool

	rootCmd := &cobra.Command{
		Use:           "slugcmplr",
		Short:         "slugcmplr helps you detach building and releasing Heroku applications",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	cmds := []func(bool) *cobra.Command{
		prepareCmd,
		compileCmd,
		releaseCmd,
		imageCmd,
		versionCmd,
		serverCmd,
		workerCmd,
	}
	for _, cmd := range cmds {
		rootCmd.AddCommand(cmd(verbose))
	}

	return rootCmd
}

type outputter interface {
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
	IsVerbose() bool
}

func outputterFromCmd(cmd *cobra.Command, verbose bool) outputter {
	return &stdOutputter{
		Err:     cmd.ErrOrStderr(),
		Out:     cmd.OutOrStdout(),
		Verbose: verbose,
	}
}

func step(cmd outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), "-----> %s\n", fmt.Sprintf(format, a...))
}

func log(cmd outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), "       %s\n", fmt.Sprintf(format, a...))
}

func wrn(cmd outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.ErrOrStderr(), " !!    %s\n", fmt.Sprintf(format, a...))
}

func dbg(cmd outputter, format string, a ...interface{}) {
	if cmd.IsVerbose() {
		log(cmd, format, a...)
	}
}

func netrcClient(cmd outputter) (*heroku.Service, error) {
	step(cmd, "Building client from .netrc...")
	netrcpath, err := netrcPath()
	if err != nil {
		wrn(cmd, "error finding .netrc file path: %v", err)
		return nil, err
	}

	rcfile, err := netrc.ParseFile(netrcpath)
	if err != nil {
		wrn(cmd, "error creating client from .netrc: %v", err)
		return nil, err
	}

	machine := rcfile.FindMachine("api.heroku.com")
	if machine == nil {
		return nil, fmt.Errorf("no .netrc entry for api.heroku.com found")
	}

	return heroku.NewService(&http.Client{
		Transport: &heroku.Transport{
			Username: machine.Login,
			Password: machine.Password,
			Transport: heroku.RoundTripWithRetryBackoff{
				MaxElapsedTimeSeconds:  15,
				InitialIntervalSeconds: 1,
				RandomizationFactor:    0.25,
				Multiplier:             2.0,
				MaxIntervalSeconds:     5,
			},
		}}), nil
}

func netrcPath() (string, error) {
	if fromEnv := os.Getenv("NETRC"); fromEnv != "" {
		return fromEnv, nil
	}

	u, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(u.HomeDir, ".netrc"), nil
}

func outputStream(cmd outputter, out io.Writer, stream string) error {
	return outputStreamAttempt(cmd, out, stream, 0)
}

func outputStreamAttempt(cmd outputter, out io.Writer, stream string, attempt int) error {
	if attempt >= 10 {
		return fmt.Errorf("failed to fetch outputStream after 5 attempts")
	}

	resp, err := http.Get(stream) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode > 399 {
		if resp.StatusCode == 404 {
			log(cmd, "Output stream 404, likely the process is still starting up. Trying again in 5s...")
			time.Sleep(5 * time.Second)

			return outputStreamAttempt(cmd, out, stream, attempt+1)
		}

		return fmt.Errorf("output stream returned HTTP status: %v", resp.Status)
	}

	scn := bufio.NewScanner(resp.Body)
	for scn.Scan() {
		fmt.Fprintf(out, "%v\n", scn.Text())
	}

	return scn.Err()
}

type stdOutputter struct {
	Out     io.Writer
	Err     io.Writer
	Verbose bool
}

func (o *stdOutputter) IsVerbose() bool {
	return o.Verbose
}

func (o *stdOutputter) OutOrStdout() io.Writer {
	if o.Out == nil {
		return os.Stdout
	}

	return o.Out
}

func (o *stdOutputter) ErrOrStderr() io.Writer {
	if o.Err == nil {
		return os.Stdout
	}

	return o.Err
}

func requireEnv(names ...string) (map[string]string, error) {
	result := map[string]string{}
	missing := []string{}

	for _, name := range names {
		v, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
		} else {
			result[name] = v
		}
	}

	if len(missing) > 0 {
		return map[string]string{}, fmt.Errorf("variables not set: %v", missing)
	}

	return result, nil
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

func initObs(cmd *cobra.Command, output slugcmplr.Outputter, service string, env map[string]string) (func(), error) {
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
				semconv.ServiceNameKey.String(service),
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
