package main

import (
	"context"
	"fmt"
	llog "log"
	"os"
	"time"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/dispatch"
	"github.com/cga1123/slugcmplr/events"
	"github.com/cga1123/slugcmplr/store"
	workers "github.com/digitalocean/go-workers2"
	"github.com/spf13/cobra"
)

func serverCmd(verbose bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a slugmcplr server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputterFromCmd(cmd, verbose)

			env, err := requireEnv(
				"PORT",
				"HONEYCOMB_WRITE_KEY",
				"SLUGCMPLR_TIMEOUT",
				"SLUGCMPLR_WEBHOOK_SECRET",
				"SLUGCMPLR_ADVERTISED_HOST",
				"REDIS_URL",
				"DATABASE_URL",
				"SLUGCMPLR_GITHUB_TOKEN",
				"SLUGCMPLR_HEROKU_LOGIN",
				"SLUGCMPLR_HEROKU_TOKEN",
			)
			if err != nil {
				return fmt.Errorf("error fetching environment: %w", err)
			}

			t, err := time.ParseDuration(env["SLUGCMPLR_TIMEOUT"])
			if err != nil {
				return fmt.Errorf("failed to parse duration: %w", err)
			}

			shutdown, err := InitObs(context.Background(), env["HONEYCOMB_WRITE_KEY"], "slugcmplr", "slugcmplr-http")
			if err != nil {
				return fmt.Errorf("error initialising o11y: %w", err)
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
				ProcessID: "bar", // TODO
				Namespace: "slugcmplr",
			}, rdb)
			if err != nil {
				return fmt.Errorf("error setting up workers: %w", err)
			}

			queries, err := store.Build(env["DATABASE_URL"])
			if err != nil {
				return fmt.Errorf("failed to build store: %w", err)
			}

			s := &slugcmplr.ServerCmd{
				RequestTimeout: t,
				Port:           env["PORT"],
				WebhookSecret:  []byte(env["SLUGCMPLR_WEBHOOK_SECRET"]),
				AdvertisedHost: env["SLUGCMPLR_ADVERTISED_HOST"],
				Events:         events.New(m, queries, dispatch.NullDispatcher()),
				Store:          queries,
				GitHub:         githubClient(env["SLUGCMPLR_GITHUB_TOKEN"]),
				Heroku:         herokuClient(env["SLUGCMPLR_HEROKU_LOGIN"], env["SLUGCMPLR_HEROKU_TOKEN"]),
			}

			return s.Execute(cmd.Context(), output)
		},
	}

	return cmd
}

func requireEnv(names ...string) (map[string]string, error) {
	result := map[string]string{}
	missing := []string{}

	for _, name := range names {
		v, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
		} else {
			result[name] = v
		}
	}

	if len(missing) > 0 {
		return map[string]string{}, fmt.Errorf("variables not set: %v", missing)
	}

	return result, nil
}
