package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func versionCmd(verbose bool) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := outputterFromCmd(cmd, verbose).OutOrStdout()

			fmt.Fprintf(out, "Build Version:   %v\n", version)
			fmt.Fprintf(out, "Build Commit:    %v\n", commit)
			fmt.Fprintf(out, "Build Date:      %v\n", date)

			return nil
		},
	}
}
