package compilesvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-github/v39/github"
	"github.com/pelletier/go-toml/v2"
)

// RepoClient contains methods which are able to retrieve metadata for a repository.
type RepoClient interface {
	DownloadURL(ctx context.Context, owner string, repo string, ref string) (string, error)
	ConfigFile(ctx context.Context, owner string, repo string, ref string) (*RepoConfig, error)
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
