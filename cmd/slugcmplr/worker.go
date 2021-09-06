package main

import (
	"context"
	"fmt"
	llog "log"
	"time"

	"github.com/cga1123/slugcmplr/dispatch"
	"github.com/cga1123/slugcmplr/events"
	"github.com/cga1123/slugcmplr/store"
	workers "github.com/digitalocean/go-workers2"
	"github.com/spf13/cobra"
)

func workerCmd(_ bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "start a slugmcplr worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := requireEnv(
				"REDIS_URL",
				"HONEYCOMB_WRITE_KEY",
				"DATABASE_URL",
				"SLUGCMPLR_GITHUB_TOKEN",
				"SLUGCMPLR_HEROKU_LOGIN",
				"SLUGCMPLR_HEROKU_TOKEN",
				"SLUGCMPLR_BASE_SERVER_URL",
				"SLUGCMPLR_WORKER_APP",
			)
			if err != nil {
				return fmt.Errorf("error fetching environment: %w", err)
			}

			shutdown, err := InitObs(
				context.Background(),
				env["HONEYCOMB_WRITE_KEY"],
				"slugcmplr",
				"slugcmplr-worker",
			)
			if err != nil {
				return fmt.Errorf("error setting up o11y: %w", err)
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()

				if err := shutdown(ctx); err != nil {
					llog.Printf("error shutting down otel: %v", err)
				}
			}()

			rdb, err := setupRedis(env["REDIS_URL"])
			if err != nil {
				return fmt.Errorf("failed to setup redis connection: %w", err)
			}

			m, err := workers.NewManagerWithRedisClient(workers.Options{
				ProcessID: "foo", // TODO
				Namespace: "slugcmplr",
			}, rdb)
			if err != nil {
				return fmt.Errorf("error setting up workers: %w", err)
			}

			s, err := store.Build(env["DATABASE_URL"])
			if err != nil {
				return fmt.Errorf("failed to build store: %w", err)
			}

			d := dispatch.NewHerokuDispatcher(
				herokuClient(env["SLUGCMPLR_HEROKU_LOGIN"], env["SLUGCMPLR_HEROKU_TOKEN"]),
				env["SLUGCMPLR_WORKER_APP"],
				env["SLUGCMPLR_BASE_SERVER_URL"],
			)

			// Setup queues and events
			events.New(m, s, d)

			m.Run()

			return nil
		},
	}

	return cmd
}
