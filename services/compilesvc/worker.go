package compilesvc

// TODO: finished compilations should cache tree_sha + target, allowing for
// skipping due to cache.  Might cause issues as builds include changes in
// config vars or buildpacks.  Make it configurable?
//
// TODO: Send events to GitHub Checks API?

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/cga1123/slugcmplr/obs"
	"github.com/cga1123/slugcmplr/proto/compileworker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/services/compilesvc/store"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
)

// RepoConfig contains the parsed data of a .slugcmplr.toml file.
type RepoConfig struct {
	Targets []string `toml:"targets"`
}

// Work registers the jobs for the compile service with the worker.
func Work(m *mux.Router, enq queue.Enqueuer, s store.Querier, repoClient RepoClient, dispatcher Dispatcher) {
	svc := &worker{
		repoClient: repoClient,
		store:      s,
		dispatcher: dispatcher,
		compileEnq: compileworker.NewCompileJSONClient("", queue.TwirpEnqueuer(enq)),
	}

	m.PathPrefix("").Handler(compileworker.NewCompileServer(svc, twirp.WithServerInterceptors(obs.TwirpOtelInterceptor())))
}

var _ compileworker.Compile = (*worker)(nil)

type worker struct {
	store      store.Querier
	repoClient RepoClient
	compileEnq compileworker.Compile
	dispatcher Dispatcher
}

// TriggerForRepository will fetch the .slugcmplr.toml configuration file for
// the given repository at the give SHA and fan-out a TriggerForTarget job for
// each declared application target.
func (w *worker) TriggerForRepository(ctx context.Context, r *compileworker.RepositoryInfo) (*compileworker.JobResponse, error) {
	config, err := w.repoClient.ConfigFile(ctx, r.Owner, r.Repository, r.CommitSha)
	if err != nil {
		return nil, twirp.InternalErrorWith(err)
	}

	for _, target := range config.Targets {
		// TODO: Should Enqueuer have a Batch(func(queue.Enqueuer)) error fn
		// which collects all enqueues and batch saves them?
		_, err := w.compileEnq.TriggerForTarget(ctx, &compileworker.TargetInfo{
			Target:     target,
			EventId:    r.EventId,
			CommitSha:  r.CommitSha,
			Owner:      r.Owner,
			Repository: r.Repository,
			TreeSha:    r.TreeSha,
		})
		if err != nil {
			// TODO retryable.
			return nil, fmt.Errorf("failed to enqueue for target: %w", err)
		}
	}

	return &compileworker.JobResponse{}, nil
}

// TriggerForTarget will kick-off a compilation for the given target
// application, using the configured Dispatcher.
func (w *worker) TriggerForTarget(ctx context.Context, r *compileworker.TargetInfo) (*compileworker.JobResponse, error) {
	if err := w.store.Create(ctx, store.CreateParams{
		EventID:    r.EventId,
		Target:     r.Target,
		CommitSha:  r.CommitSha,
		TreeSha:    r.TreeSha,
		Owner:      r.Owner,
		Repository: r.Repository,
	}); err != nil {
		return nil, err // TODO: how can retries be handled gracefully?
	}

	tok, err := token()
	if err != nil {
		// TODO: retryable
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	if err := w.store.CreateToken(ctx, store.CreateTokenParams{
		EventID: r.EventId,
		Target:  r.Target,
		Token:   tok,
	}); err != nil {
		// TODO: what if there is already a token that is active?
		return nil, err
	}

	if err := w.dispatcher.Dispatch(ctx, tok); err != nil {
		// TODO: retryable!
		return nil, err
	}

	return &compileworker.JobResponse{}, nil
}

func token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("error creating token: %w", err)
	}

	return fmt.Sprintf("%x", b), nil
}
