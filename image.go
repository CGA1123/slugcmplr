package slugcmplr

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/cga1123/slugcmplr/buildpack"
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

// Dockerfile builds the contents of the dockerfile based on a template.
func Dockerfile(baseImage, appDirectory, command string) ([]byte, error) {
	t, err := template.New("").Parse(dockerfileTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to build Dockerfile template: %w", err)
	}

	b := &bytes.Buffer{}
	if err := t.Execute(b, templateVars{
		BaseImage:    baseImage,
		AppDirectory: appDirectory,
		Command:      command,
	}); err != nil {
		return nil, fmt.Errorf("failed to execute Dockerfile template: %w", err)
	}

	return b.Bytes(), nil
}

// ImageCmd wraps up all the information required to build a Docker image from
// a slugcmplr compiled application.
type ImageCmd struct {
	BuildDir string
	Image    string
	Stack    string
	Command  string
}

// Execute creates a new Docker image based on a Dockerfile that will be
// written to buildDir/Dockerfile.
//
// The image is built with buildDir as it's build context and will copy in the
// contents of buildDir/app into /app.
func (i *ImageCmd) Execute(ctx context.Context, out Outputter) error {
	image := StackImage(i.Image, i.Stack)
	appDir := filepath.Join(i.BuildDir, buildpack.AppDir)
	dockerfile, err := Dockerfile(image, appDir, i.Command)
	if err != nil {
		return fmt.Errorf("failed template Dockerfile: %w", err)
	}

	dockerfilePath := filepath.Join(i.BuildDir, "Dockerfile")

	// #nosec G306
	if err := os.WriteFile(dockerfilePath, dockerfile, 0644); err != nil {
		return fmt.Errorf("failed to create Dockerfile: %w", err)
	}

	dockerBuild := exec.CommandContext(ctx, "docker", "build",
		"--quiet",
		"--file", dockerfilePath,
		i.BuildDir,
	) // #nosec G204
	dockerBuild.Stderr, dockerBuild.Stdout = out.ErrOrStderr(), out.OutOrStdout()

	return dockerBuild.Run()
}
