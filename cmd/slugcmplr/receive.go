package main

import (
	"context"
	"fmt"
	"time"

	"github.com/cga1123/slugcmplr"
	"github.com/spf13/cobra"
)

func receiveCmd(verbose bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "receive",
		Short: "Receives a compilation request for the slugcmplr server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			output := outputterFromCmd(cmd, verbose)
			env, err := requireEnv(
				"SLUGCMPLR_RECEIVE_TOKEN",
				"SLUGCMPLR_BASE_SERVER_URL",
			)
			if err != nil {
				return fmt.Errorf("missing env: %w", err)
			}

			log(output, "base_url: %v", env["SLUGCMPLR_BASE_SERVER_URL"])

			r := &slugcmplr.ReceiveCmd{
				BaseURL:      env["SLUGCMPLR_BASE_SERVER_URL"],
				ReceiveToken: env["SLUGCMPLR_RECEIVE_TOKEN"],
			}

			// Set a default 30min timeout
			// Should this be configurable by env var?
			ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
			defer cancel()

			return r.Execute(ctx, output)
		},
	}

	return cmd
}
