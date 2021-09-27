package main

import (
	"fmt"
	"os"

	"github.com/cga1123/slugcmplr"
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
			)
			if err != nil {
				return fmt.Errorf("error fetching environment: %w", err)
			}

			s := &slugcmplr.ServerCmd{
				Port:        env["PORT"],
				Environment: env["SLUGCMPLR_ENV"],
			}

			return s.Execute(cmd.Context(), output)
		},
	}

	return cmd
}

func defaultEnv() error {
	defaults := map[string]string{
		"SLUGCMPLR_ENV": "development",
	}
	for k, v := range defaults {
		if _, ok := os.LookupEnv(k); ok {
			continue
		}

		if err := os.Setenv(k, v); err != nil {
			return err
		}
	}

	return nil
}
