package jobs

import (
	context "context"
	"crypto/rand"
	fmt "fmt"
	"sort"

	"github.com/cga1123/slugcmplr/store"
	"github.com/google/go-github/v39/github"
	heroku "github.com/heroku/heroku-go/v5"
)

var _ Jobs = (*jobsSvc)(nil)

type jobsSvc struct {
	s store.Querier
	g *github.Client
	h *heroku.Service
}

// NewService returns a Twirp Jobs service.
func NewService(s store.Querier, g *github.Client, h *heroku.Service) Jobs {
	return &jobsSvc{s: s, g: g, h: h}
}

func (j *jobsSvc) Receive(ctx context.Context, r *ReceiveRequest) (*ReceiveResponse, error) {
	g, h := j.g, j.h

	build, err := j.s.GetBuildRequestFromReceiveToken(ctx, r.ReceiveToken)
	if err != nil {
		return nil, fmt.Errorf("error fetching build request: %w", err)
	}

	configVars, err := h.ConfigVarInfoForApp(ctx, build.Target)
	if err != nil {
		return nil, fmt.Errorf("error fetching config vars: %w", err)
	}

	env := make(map[string]string, len(configVars))
	for k, v := range configVars {
		if v == nil {
			continue
		}

		env[k] = *v
	}

	info, err := h.AppInfo(ctx, build.Target)
	if err != nil {
		return nil, fmt.Errorf("error fetching app info: %w", err)
	}

	bpi, err := h.BuildpackInstallationList(ctx, build.Target, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app buildpacks: %w", err)
	}
	sort.Slice(bpi, func(a, b int) bool {
		return bpi[a].Ordinal < bpi[b].Ordinal
	})

	bps := make([]string, len(bpi))
	for i, bp := range bpi {
		bps[i] = bp.Buildpack.URL
	}

	// GetArchiveLink will return a link with a 5min token for private repos.
	url, _, err := g.Repositories.GetArchiveLink(
		ctx,
		build.Owner,
		build.Repository,
		github.Tarball,
		&github.RepositoryContentGetOptions{Ref: build.CommitSha},
		false,
	)
	if err != nil {
		return nil, fmt.Errorf("error fetching archive link: %w", err)
	}

	token, err := token()
	if err != nil {
		return nil, fmt.Errorf("error generating token: %w", err)
	}

	if _, err := j.s.CreateBuildToken(ctx, store.CreateBuildTokenParams{
		BuildRequestID:     build.ID,
		BuildRequestTarget: build.Target,
		Token:              token,
	}); err != nil {
		return nil, fmt.Errorf("error storing token: %w", err)
	}

	return &ReceiveResponse{
		Token:         token,
		SourceUrl:     url.String(),
		Stack:         info.Stack.Name,
		SourceVersion: build.CommitSha,
		BuildpackUrn:  bps,
		ConfigVars:    env,
	}, nil
}

// TODO: Implement ClaimToken.
func (j *jobsSvc) ClaimToken(_ context.Context, _ *ClaimTokenRequest) (*ClaimTokenResponse, error) {
	return nil, nil
}

// TODO: Implement Upload.
func (j *jobsSvc) Upload(_ context.Context, _ *UploadRequest) (*UploadResponse, error) {
	return nil, nil
}

func token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("error creating token: %w", err)
	}

	return fmt.Sprintf("%x", b), nil
}
