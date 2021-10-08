package main

import (
	"context"
	"fmt"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/spf13/cobra"
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
			w := &slugcmplr.WorkerCmd{
				Dequeuer: q,
				Enqueuer: q,
				Queues:   map[string]int{"default": 1},
				Router:   mux.NewRouter(),
			}

			return w.Execute(cmd.Context(), output)
		},
	}

	return cmd
}
