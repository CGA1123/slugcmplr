package compilesvc

import (
	"context"
	"fmt"

	heroku "github.com/heroku/heroku-go/v5"
)

// Dispatcher contains a single Dispatch method, which is used to schedule
// compilation jobs.
type Dispatcher interface {
	// Dispatch dispatches a particular job, given a unique one-time token
	// which the receiver can use to initiate the connection and acquire the
	// context required to fulfill the request.
	Dispatch(context.Context, string) error
}

// HerokuDispatcher implements the Dispatcher interface backed by detached
// Heroku dynos.
type HerokuDispatcher struct {
	c   *heroku.Service
	app string
	url string
}

// NullDispatcher returns a Dispatcher that always errors.
func NullDispatcher() Dispatcher {
	return &nullDispatcher{}
}

type nullDispatcher struct{}

func (n *nullDispatcher) Dispatch(_ context.Context, _ string) error {
	return fmt.Errorf("the null dispatcher does not dispatch any jobs")
}

// NewHerokuDispatcher creates a Dispatcher which schedules a detached Heroku
// dyno.
func NewHerokuDispatcher(c *heroku.Service, app, url string) *HerokuDispatcher {
	return &HerokuDispatcher{c: c, app: app, url: url}
}

// Dispatch creates a new detached Heroku dyno.
func (h *HerokuDispatcher) Dispatch(ctx context.Context, token string) error {
	_, err := h.c.DynoCreate(ctx, h.app, heroku.DynoCreateOpts{
		Attach:  heroku.Bool(false), // run:detached
		Command: "slugcmplr receive",
		Env: map[string]string{
			"SLUGCMPLR_RECEIVE_TOKEN":   token,
			"SLUGCMPLR_BASE_SERVER_URL": h.url,
		},
	})
	if err != nil {
		return fmt.Errorf("error creating dyno: %w", err)
	}

	return nil
}
