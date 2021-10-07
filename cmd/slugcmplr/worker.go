package main

import "github.com/spf13/cobra"

func workerCmd(_ bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "start a slugcmplr worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	return cmd
}
