// TODO: finished compilations should cache tree_sha + target, allowing for
// skipping due to cache.  Might cause issues as builds include changes in
// config vars or buildpacks.  Make it configurable?
//
// TODO: Send events to GitHub Checks API?
package compilesvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/cga1123/slugcmplr/obs"
	"github.com/cga1123/slugcmplr/proto/compileworker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/google/go-github/v39/github"
	"github.com/gorilla/mux"
	"github.com/pelletier/go-toml/v2"
	"github.com/twitchtv/twirp"
)

// RepoConfig contains the parsed data of a .slugcmplr.toml file.
type RepoConfig struct {
	Targets []string `toml:"targets"`
}

type githubRepoClient struct {
	c *github.Client
}

// NewGitHubRepoClient creates a RepoClient that is backed by the GitHub API.
func NewGitHubRepoClient(c *github.Client) RepoClient {
	return &githubRepoClient{c: c}
}

// DownloadURL will return a URL to the tarball for the given repo.
// For private repos, the returned URL will contain a token to access the
// contents valid for 5mins.
func (g *githubRepoClient) DownloadURL(ctx context.Context, owner, repo, ref string) (string, error) {
	url, _, err := g.c.Repositories.GetArchiveLink(
		ctx,
		owner,
		repo,
		github.Tarball,
		&github.RepositoryContentGetOptions{Ref: ref},
		false,
	)
	if err != nil {
		return "", fmt.Errorf("error fetching archive link: %w", err)
	}

	return url.String(), nil
}

// ConfigFile will fetch and parse the .slugcmplr.toml file for the repo.
func (g *githubRepoClient) ConfigFile(ctx context.Context, owner, repo, ref string) (*RepoConfig, error) {
	content, _, _, err := g.c.Repositories.GetContents(
		ctx,
		owner,
		repo,
		".slugcmplr.toml",
		&github.RepositoryContentGetOptions{Ref: ref},
	)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, fmt.Errorf("metadata path is not a file")
	}

	file, err := content.GetContent()
	if err != nil {
		return nil, fmt.Errorf("error fetching content: %w", err)
	}

	config := &RepoConfig{}
	if err := toml.NewDecoder(strings.NewReader(file)).Decode(config); err != nil {
		return nil, fmt.Errorf("error decoding: %w", err)
	}

	return config, nil
}

var _ compileworker.Compile = (*worker)(nil)

type RepoClient interface {
	DownloadURL(ctx context.Context, owner string, repo string, ref string) (string, error)
	ConfigFile(ctx context.Context, owner string, repo string, ref string) (*RepoConfig, error)
}

// Work registers the jobs for the compile service with the worker.
func Work(m *mux.Router, enq queue.Enqueuer) {
	svc := &worker{
		compileEnq: compileworker.NewCompileJSONClient("", queue.TwirpEnqueuer(enq)),
	}

	m.PathPrefix("").Handler(compileworker.NewCompileServer(svc, twirp.WithServerInterceptors(obs.TwirpOtelInterceptor())))
}

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
		// TODO: should TriggerForTargets be a thing to enable batch enqueueing?
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
func (w *worker) TriggerForTarget(context.Context, *compileworker.TargetInfo) (*compileworker.JobResponse, error) {
	// TODO: implement this.
	return nil, fmt.Errorf("nyi")
}
