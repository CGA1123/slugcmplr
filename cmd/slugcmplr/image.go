package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cga1123/slugcmplr"
	"github.com/spf13/cobra"
)

func imageCmd(verbose bool) *cobra.Command {
	var buildDir, img, command string

	cmd := &cobra.Command{
		Use:   "image",
		Short: "build a container from your compiled application",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputterFromCmd(cmd, verbose)

			dbg(output, "buildDir: %v", buildDir)
			dbg(output, "image: %v", img)
			dbg(output, "command: %v", command)

			m, err := os.Open(filepath.Join(buildDir, "meta.json"))
			if err != nil {
				return fmt.Errorf("failed to read metadata: %w", err)
			}
			defer m.Close() // nolint:errcheck

			c := &Compile{}
			if err := json.NewDecoder(m).Decode(c); err != nil {
				return fmt.Errorf("failed to decode metadata: %w", err)
			}

			i := &slugcmplr.ImageCmd{
				BuildDir: buildDir,
				Image:    img,
				Command:  command,
				Stack:    c.Stack,
			}

			return i.Execute(cmd.Context(), output)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	cmd.Flags().StringVar(&command, "cmd", "", "The command (CMD) to run by default")
	cmd.MarkFlagRequired("cmd") // nolint:errcheck

	cmd.Flags().StringVar(
		&img,
		"image",
		fmt.Sprintf("heroku/heroku:%s", slugcmplr.StackNumberReplacePattern),
		fmt.Sprintf(
			"Override docker image to use, include %s in order to substitute the stack number",
			slugcmplr.StackNumberReplacePattern,
		),
	)

	return cmd
}
