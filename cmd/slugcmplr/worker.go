package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/obs"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/services/compilesvc"
	cstore "github.com/cga1123/slugcmplr/services/compilesvc/store"
	"github.com/cga1123/slugcmplr/services/pingsvc"
	"github.com/google/go-github/v39/github"
	"github.com/gorilla/mux"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
)

func workerCmd(verbose bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "start a slugcmplr worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputterFromCmd(cmd, verbose)

			if err := defaultEnv(); err != nil {
				return fmt.Errorf("failed to set env defaults: %w", err)
			}

			env, err := requireEnv(
				"SLUGCMPLR_ENV",
				"DATABASE_URL",
				"SLUGCMPLR_GITHUB_TOKEN",
				"SLUGCMPLR_HEROKU_LOGIN",
				"SLUGCMPLR_HEROKU_TOKEN",
				"SLUGCMPLR_ADVERTISED_HOST",
				"SLUGCMPLR_WORKER_APP",
			)
			if err != nil {
				return fmt.Errorf("error fetching environment: %w", err)
			}

			closer, err := initObs(cmd, output, "slugcmplr-worker", env)
			if err != nil {
				return err
			}
			defer closer()

			config, err := pgxpool.ParseConfig(env["DATABASE_URL"])
			if err != nil {
				return fmt.Errorf("error parsing connstr: %w", err)
			}

			// heroku-postgresql:hobby-dev only has 20 connections available.
			config.MaxConns = 8
			config.MinConns = 8

			pool, err := pgxpool.ConnectConfig(context.Background(), config)
			if err != nil {
				return fmt.Errorf("error creating db connection pool: %w", err)
			}

			q := queue.New(pool)
			r := mux.NewRouter()

			gh := github.NewClient(
				oauth2.NewClient(
					context.WithValue(context.Background(), oauth2.HTTPClient, otelhttp.DefaultClient),
					oauth2.StaticTokenSource(
						&oauth2.Token{AccessToken: env["SLUGCMPLR_GITHUB_TOKEN"]},
					),
				),
			)

			h := heroku.NewService(&http.Client{
				Transport: otelhttp.NewTransport(
					&heroku.Transport{
						Username: env["SLUGCMPLR_HEROKU_LOGIN"],
						Password: env["SLUGCMPLR_HEROKU_TOKEN"],
					},
				),
			})

			pingsvc.Work(r)

			baseURL := fmt.Sprintf("https://%v/", env["SLUGCMPLR_ADVERTISED_HOST"])
			compilesvc.Work(r, q,
				cstore.New(obs.NewDB(pool)),
				compilesvc.NewGitHubRepoClient(gh),
				compilesvc.NewHerokuDispatcher(h, env["SLUGCMPLR_WORKER_APP"], baseURL),
			)

			w := &slugcmplr.WorkerCmd{
				Queues:   map[string]int{"default": 1},
				Dequeuer: q,
				Router:   r,
			}

			return w.Execute(cmd.Context(), output)
		},
	}

	return cmd
}
