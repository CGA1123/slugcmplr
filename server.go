package slugcmplr

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/services/pingsvc"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	production  = "production"
	development = "development"
	test        = "test"
)

// ServerCmd wraps up all the information required to start a slugcmplr HTTP
// server.
type ServerCmd struct {
	// Port is the port to listen to, the server will bind to the 0.0.0.0
	// interface by default.
	Port string

	// Environment contains the current deployment environment, e.g. "production"
	Environment string
}

// Execute starts a slugcmplr server, blocking untile a SIGTERM/SIGINT is
// received.
func (s *ServerCmd) Execute(ctx context.Context, out Outputter) error {
	return runServer(ctx, out, s.Port, s.Router())
}

// Router builds a *mux.Router for slugcmplr.
func (s *ServerCmd) Router() *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://imgs.xkcd.com/comics/compiling.png", http.StatusFound)
	})

	pingsvc := ping.NewPingServer(&pingsvc.Service{}, twirp.WithServerInterceptors(twirpObs()))
	r.PathPrefix(ping.PingPathPrefix).Handler(pingsvc)

	return r
}

func twirpObs() twirp.Interceptor {
	return func(n twirp.Method) twirp.Method {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			span := trace.SpanFromContext(ctx)

			pkg, _ := twirp.PackageName(ctx)
			svc, _ := twirp.ServiceName(ctx)
			mtd, _ := twirp.MethodName(ctx)
			fqn := pkg + "." + mtd + "/" + mtd

			span.SetAttributes(
				semconv.RPCSystemKey.String("twirp"),
				semconv.RPCServiceKey.String(svc),
				semconv.RPCMethodKey.String(mtd),
				attribute.String("rpc.package", pkg),
				attribute.String("rpc.fqn", fqn),
			)

			res, err := n(ctx, req)
			if err != nil {
				var terr twirp.Error
				if errors.As(err, &terr) {
					span.SetAttributes(
						attribute.String("rpc.error_code", string(terr.Code())),
						attribute.String("rpc.error_message", terr.Msg()),
					)
				} else {
					span.SetAttributes(
						attribute.String("rpc.error_code", "other"),
						attribute.String("rpc.error_message", err.Error()),
					)
				}
			}

			return res, err
		}
	}
}

// nolint:unused
func loggingHandler(out io.Writer) func(n http.Handler) http.Handler {
	return func(n http.Handler) http.Handler {
		return handlers.LoggingHandler(out, n)
	}
}

// timeoutHandler will set a timeout on the request from having read the
// headers until writing the full response body.
//
// For most requests this should be 30s or less. Heroku will close any
// connection that has not started writing responses within 30s.
//
// See: https://devcenter.heroku.com/articles/http-routing#timeouts
//
// nolint:unused
func timeoutHandler(t time.Duration) func(http.Handler) http.Handler {
	return func(n http.Handler) http.Handler {
		return http.TimeoutHandler(n, t, http.StatusText(http.StatusServiceUnavailable))
	}
}

func runServer(ctx context.Context, out Outputter, port string, r *mux.Router) error {
	r.Use(
		loggingHandler(out.OutOrStdout()),
		otelmux.Middleware("slugcmplr-http"),
	)

	// Default Handler 404s
	r.PathPrefix("/").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
	)

	server := &http.Server{
		Addr: "0.0.0.0:" + port,

		// ReadTimeout sets a timeout from connection open until fully
		// request-body read. Mitigating slow client attacks.
		ReadTimeout: time.Second * 10,
		Handler:     r,
	}

	errorC := make(chan error, 1)
	shutdownC := make(chan os.Signal, 1)

	go func(errC chan<- error) {
		errC <- server.ListenAndServe()
	}(errorC)

	signal.Notify(shutdownC, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errorC:
		if err != nil && err != http.ErrServerClosed {
			return err
		}

		return nil
	case <-shutdownC:
		return shutdown(server)
	case <-ctx.Done():
		return shutdown(server)
	}
}

func shutdown(server *http.Server) error {
	// Heroku Dynos are given 30s to shutdown gracefully.
	ctx, cancel := context.WithTimeout(context.Background(), 29*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

// nolint:unused
func (s *ServerCmd) inProduction() bool {
	return s.Environment == production
}

// nolint:unused
func (s *ServerCmd) inDevelopment() bool {
	return s.Environment == development
}

// nolint:unused
func (s *ServerCmd) inTest() bool {
	return s.Environment == test
}
