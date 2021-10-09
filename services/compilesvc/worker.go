package compilesvc

// TODO: finished compilations should cache tree_sha + target, allowing for
// skipping due to cache.  Might cause issues as builds include changes in
// config vars or buildpacks.  Make it configurable?
//
// TODO: Send events to GitHub Checks API?

import (
	"context"
	"fmt"

	"github.com/cga1123/slugcmplr/obs"
	"github.com/cga1123/slugcmplr/proto/compileworker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
)

// RepoConfig contains the parsed data of a .slugcmplr.toml file.
type RepoConfig struct {
	Targets []string `toml:"targets"`
}

// Work registers the jobs for the compile service with the worker.
func Work(m *mux.Router, enq queue.Enqueuer) {
	svc := &worker{
		compileEnq: compileworker.NewCompileJSONClient("", queue.TwirpEnqueuer(enq)),
	}

	m.PathPrefix("").Handler(compileworker.NewCompileServer(svc, twirp.WithServerInterceptors(obs.TwirpOtelInterceptor())))
}

var _ compileworker.Compile = (*worker)(nil)

type worker struct {
	repoClient RepoClient
	compileEnq compileworker.Compile
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
			// TODO this should be retryable.
			return nil, fmt.Errorf("failed to enqueue for target: %w", err)
		}
	}

	return &compileworker.JobResponse{}, nil
}

// TriggerForTarget will kick-off a compilation for the given target
// application, using the configured Dispatcher.
// TODO: This should create the target job in a DB and set up a dispatcher.
// TODO: A receive token should be minted and traded for a normal token during initial comms.
func (w *worker) TriggerForTarget(ctx context.Context, r *compileworker.TargetInfo) (*compileworker.JobResponse, error) {
	return &compileworker.JobResponse{}, nil
}
