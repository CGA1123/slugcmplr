package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/processfile"
	"github.com/spf13/cobra"
)

func imageCmd(verbose bool) *cobra.Command {
	var buildDir, img, command, process string
	var noBuild bool

	cmd := &cobra.Command{
		Use:   "image",
		Short: "build a container from your compiled application",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputterFromCmd(cmd, verbose)

			dbg(output, "buildDir: %v", buildDir)
			dbg(output, "image:    %v", img)
			dbg(output, "command:  %v", command)
			dbg(output, "process:  %v", process)

			if process == "" && command == "" {
				return fmt.Errorf("either --cmd or --process must be provided")
			}

			if process != "" {
				c, err := commandFromProcfile(buildDir, process)
				if err != nil {
					return fmt.Errorf("error determining command from Procfile: %w", err)
				}

				command = c
			}

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
				NoBuild:  noBuild,
			}

			return i.Execute(cmd.Context(), output)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	cmd.Flags().StringVar(&command, "cmd", "", "The command (CMD) to run by default")
	cmd.Flags().StringVar(
		&process,
		"process",
		"",
		"The command (CMD) to run by default, based on Procfile entries. Takes precedence over --cmd",
	)

	cmd.Flags().BoolVar(&noBuild, "no-build", false, "Skip building the image, only generate the Dockerfile")
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

func commandFromProcfile(buildDir, process string) (string, error) {
	f, err := os.Open(filepath.Join(buildDir, buildpack.AppDir, "Procfile"))
	if err != nil {
		return "", fmt.Errorf("error opening Procfile: %w", err)
	}
	defer f.Close() // nolint:errcheck

	pf, err := processfile.Read(f)
	if err != nil {
		return "", fmt.Errorf("error reading Procfile: %w", err)
	}

	c, ok := pf.Entrypoint(process)
	if !ok {
		return "", fmt.Errorf("%v is not defined in Procfile (%v available)", process, strings.Join(pf.Processes(), ", "))
	}

	return c, nil
}
