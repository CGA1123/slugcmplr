package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/spf13/cobra"
)

const dockerfileTemplate = `FROM {{.BaseImage}}
RUN groupadd -r dyno && useradd --no-log-init -r -g dyno u1123
WORKDIR /app
COPY {{.AppDirectory}} /app
RUN chown -R u1123:dyno /app
USER u1123

CMD {{.Command}}
`

type templateVars struct {
	BaseImage    string
	AppDirectory string
	Command      string
}

func image(ctx context.Context, out Outputter, buildDir, image, cmd string) error {
	m, err := os.Open(filepath.Join(buildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}
	defer m.Close() // nolint:errcheck

	c := &Compile{}
	if err := json.NewDecoder(m).Decode(c); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	t, err := template.New("").Parse(dockerfileTemplate)
	if err != nil {
		return fmt.Errorf("failed to build Dockerfile template: %w", err)
	}

	f, err := os.Create(filepath.Join(buildDir, "Dockerfile"))
	if err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}
	defer f.Close() // nolint:errcheck

	if err := t.Execute(f, templateVars{
		BaseImage:    strings.ReplaceAll(image, "%stack%", strings.TrimPrefix(c.Stack, "heroku-")),
		AppDirectory: buildpack.AppDir,
		Command:      cmd,
	}); err != nil {
		return fmt.Errorf("failed to execute Dockerfile template: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}

	dockerBuild := exec.CommandContext(ctx, "docker", "build",
		"--quiet",
		"--file", filepath.Join(buildDir, "Dockerfile"),
		buildDir,
	) // #nosec G204
	dockerBuild.Stderr, dockerBuild.Stdout = out.ErrOrStderr(), out.OutOrStdout()

	return dockerBuild.Run()
}

func imageCmd(verbose bool) *cobra.Command {
	var buildDir, img, command string

	cmd := &cobra.Command{
		Use:   "image",
		Short: "build a container from your compiled application",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := OutputterFromCmd(cmd, verbose)

			dbg(output, "buildDir: %v", buildDir)

			return image(cmd.Context(), output, buildDir, img, command)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	cmd.Flags().StringVar(&command, "cmd", "", "The command (CMD) to run by default")
	cmd.MarkFlagRequired("cmd") // nolint:errcheck

	cmd.Flags().StringVar(&img, "image", "heroku/heroku:%stack%", "Override docker image to use, include %stack% in order to substitute the stack number")

	return cmd
}
