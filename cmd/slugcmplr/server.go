package main

import (
	"context"
	"fmt"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/obs"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/store"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/spf13/cobra"
)

func serverCmd(verbose bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a slugmcplr server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputterFromCmd(cmd, verbose)

			if err := defaultEnv(); err != nil {
				return fmt.Errorf("failed to set env defaults: %w", err)
			}

			env, err := requireEnv(
				"PORT",
				"SLUGCMPLR_ENV",
				"DATABASE_URL",
				"SLUGCMPLR_WEBHOOK_SECRET",
			)
			if err != nil {
				return fmt.Errorf("error fetching environment: %w", err)
			}

			closer, err := initObs(cmd, output, "slugcmplr-http", env)
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

			s := &slugcmplr.ServerCmd{
				Port:          env["PORT"],
				Environment:   env["SLUGCMPLR_ENV"],
				Store:         store.New(obs.NewDB(pool)),
				Enqueuer:      queue.New(pool),
				WebhookSecret: []byte(env["SLUGCMPLR_WEBHOOK_SECRET"]),
			}

			return s.Execute(cmd.Context(), output)
		},
	}

	return cmd
}
