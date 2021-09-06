package slugcmplr

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cga1123/slugcmplr/events"
	"github.com/cga1123/slugcmplr/services/jobs"
	"github.com/cga1123/slugcmplr/store"
	"github.com/google/go-github/v39/github"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/pelletier/go-toml/v2"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ServerCmd wraps up all the information required to start a slugcmplr HTTP
// server.
type ServerCmd struct {
	// AdvertisedHost is the publicly routable host that this server can be
	// accessed on.
	//
	// This is used as the base URL passed on to the build process so that it
	// can connect back.
	AdvertisedHost string

	// Port is the port to listen to, the server will bind to the 0.0.0.0
	// interface by default.
	Port string

	// RequestTimeout is the global incoming request timeout for the server.
	RequestTimeout time.Duration

	// WebhookSecret is the secret used to authenticate incoming webhooks from
	// GitHub.
	WebhookSecret []byte

	// Store implements the data storage interface for slugcmplr.
	Store store.Querier

	// Events implements the async messaging interface for slugcmplr.
	Events events.Events

	// GitHub allows access to the GitHub API
	GitHub *github.Client

	// Heroku allows access to the Heroku API
	Heroku *heroku.Service
}

// Execute starts a slugcmplr server, blocking untile a SIGTERM/SIGINT is
// received.
func (s *ServerCmd) Execute(_ context.Context, _ Outputter) error {
	return runServer(s.Port, s.RequestTimeout, s.Router())
}

// Router builds a *mux.Router for slugcmplr.
func (s *ServerCmd) Router() *mux.Router {
	r := mux.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://imgs.xkcd.com/comics/compiling.png", http.StatusFound)
	})

	r.HandleFunc("/github/events", s.githubEventHandler).Methods(http.MethodPost)
	r.PathPrefix(jobs.JobsPathPrefix).Handler(jobs.NewJobsServer(jobs.NewService(s.Store, s.GitHub, s.Heroku)))

	return r
}

func loggingHandler(n http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, n)
}

func timeoutHandler(t time.Duration) func(http.Handler) http.Handler {
	return func(n http.Handler) http.Handler {
		return http.TimeoutHandler(n, t, http.StatusText(http.StatusServiceUnavailable))
	}
}

func runServer(port string, timeout time.Duration, r *mux.Router) error {
	r.Use(
		loggingHandler,
		otelmux.Middleware("slugcmplr-http"),
		timeoutHandler(timeout),
	)

	// Default Handler
	r.PathPrefix("/").HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		},
	)

	server := &http.Server{
		Addr:         "0.0.0.0:" + port,
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      r,
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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		return server.Shutdown(ctx)
	}
}

func (s *ServerCmd) githubEventHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	span.SetAttributes(
		attribute.String("hook.delivery_id", github.DeliveryID(r)),
		attribute.String("hook.event", github.WebHookType(r)),
	)

	// Validate that we received a payload from an authenticated source.
	payload, err := github.ValidatePayload(r, s.WebhookSecret)
	if err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "invalid payload"),
		)

		w.WriteHeader(http.StatusForbidden)
		return
	}

	deliveryID, err := uuid.Parse(github.DeliveryID(r))
	if err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "bad delivery_id"),
		)

		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Only listen to push events.
	hookType := github.WebHookType(r)
	if hookType != "push" {
		span.SetAttributes(
			attribute.String("hook.resolution", "skipped"),
			attribute.String("hook.resolution_reason", "webhook type"),
		)

		w.WriteHeader(http.StatusAccepted)
		return
	}

	rawEvent, err := github.ParseWebHook(hookType, payload)
	if err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "parsing payload"),
		)

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	event, ok := rawEvent.(*github.PushEvent)
	if !ok {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "coercion to push event payload"),
		)

		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	commitSHA := event.GetHeadCommit().GetID()
	treeSHA := event.GetHeadCommit().GetTreeID()
	ref := event.GetRef()
	repo := event.GetRepo().GetName()
	owner := event.GetRepo().GetOwner().GetName()

	span.SetAttributes(
		attribute.String("hook.commit_sha", commitSHA),
		attribute.String("hook.tree_sha", treeSHA),
		attribute.String("hook.ref", ref),
		attribute.String("hook.owner", owner),
		attribute.String("hook.repository", repo),
	)

	if !isDefaultBranch(event) {
		span.SetAttributes(
			attribute.String("hook.resolution", "skipped"),
			attribute.String("hook.resolution_reason", "not default branch"),
		)

		w.WriteHeader(http.StatusAccepted)
		return
	}

	config, err := fetchConfig(ctx, s.GitHub, span, owner, repo, commitSHA)
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)

		return
	}

	id, err := s.Store.CreateBuildRequest(r.Context(), store.CreateBuildRequestParams{
		ID:         deliveryID,
		Targets:    config.Targets,
		CommitSha:  commitSHA,
		TreeSha:    treeSHA,
		Owner:      owner,
		Repository: repo,
	})
	if err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "database error"),
		)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := s.Events.RequestCompilation(ctx, id.String()); err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "enqueueing error"),
		)

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	span.SetAttributes(attribute.String("hook.resolution", "succeeded"))

	w.WriteHeader(http.StatusAccepted)
}

func isDefaultBranch(event *github.PushEvent) bool {
	return strings.TrimPrefix(event.GetRef(), "refs/heads/") == event.GetRepo().GetDefaultBranch()
}

type configFile struct {
	Targets []string `toml:"targets"`
}

func fetchConfig(ctx context.Context, c *github.Client, span trace.Span, owner, repo, sha string) (*configFile, error) {
	content, _, _, err := c.Repositories.GetContents(
		ctx,
		owner,
		repo,
		".slugcmplr.toml",
		&github.RepositoryContentGetOptions{Ref: sha},
	)
	if err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "failed to fetch metadata file"),
		)

		return nil, err
	}
	if content == nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "metadata path is not a file"),
		)

		return nil, fmt.Errorf("metadata path is not a file")
	}

	file, err := content.GetContent()
	if err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "error decoding content from response"),
		)

		return nil, fmt.Errorf("error fetching content: %w", err)
	}

	config := &configFile{}
	if err := toml.NewDecoder(strings.NewReader(file)).Decode(config); err != nil {
		span.SetAttributes(
			attribute.String("hook.resolution", "failed"),
			attribute.String("hook.resolution_reason", "error decoding content into toml"),
		)

		return nil, fmt.Errorf("error decoding: %w", err)
	}

	return config, nil
}
