package slugcmplr

import (
	"context"
	"fmt"

	heroku "github.com/heroku/heroku-go/v5"
)

// ReleaseCmd wraps up all the information required to release a slug that has
// been uploaded to a Heroku application.
type ReleaseCmd struct {
	Heroku      *heroku.Service
	Application string
	SlugID      string
	Commit      string
}

// ReleaseInfo contains the ID and OutputStreamURL of an attempted release.
type ReleaseInfo struct {
	ID              string
	OutputStreamURL *string
}

// Execute will attempt to release a given slug to an application, returning
// the release ID and and OutputStreamURL, if there is one.
func (r *ReleaseCmd) Execute(ctx context.Context, _ Outputter) (*ReleaseInfo, error) {
	release, err := r.Heroku.ReleaseCreate(ctx, r.Application, heroku.ReleaseCreateOpts{
		Slug:        r.SlugID,
		Description: heroku.String(fmt.Sprintf("Deployed %v", r.Commit[:8])),
	})
	if err != nil {
		return nil, fmt.Errorf("error release slug: %w", err)
	}

	return &ReleaseInfo{ID: release.ID, OutputStreamURL: release.OutputStreamURL}, nil
}
