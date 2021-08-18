package buildpack

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	AppDir         = "app"
	EnvironmentDir = "environment"
	BuildpacksDir  = "buildpacks"
)

type Build struct {
	BuildDir      string
	CacheDir      string
	Stack         string
	SourceVersion string
	Stdout        io.Writer
	Stderr        io.Writer
}

type Buildpack struct {
	URL       string `json:"url"`
	Directory string `json:"directory"`
}

func environ(b *Build) []string {
	return append(os.Environ(), "STACK="+b.Stack, "SOURCE_VERSION="+b.SourceVersion)
}

func (b *Buildpack) Detect(ctx context.Context, build *Build) (string, bool, error) {
	detect := filepath.Join(build.BuildDir, BuildpacksDir, b.Directory, "bin", "detect")
	stdout := &strings.Builder{}

	detectCmd := exec.CommandContext(ctx, detect, filepath.Join(build.BuildDir, AppDir)) // #nosec G204
	detectCmd.Env = environ(build)
	detectCmd.Stderr, detectCmd.Stdout = build.Stderr, stdout

	if err := detectCmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", false, nil
		}

		return "", false, err
	}

	return strings.TrimSpace(stdout.String()), true, nil
}

func (b *Buildpack) Compile(ctx context.Context, exports []*Buildpack, build *Build) error {
	compile := filepath.Join(build.BuildDir, BuildpacksDir, b.Directory, "bin", "compile")
	commandParts := []string{}

	// exports
	for _, export := range exports {
		dir, ok, err := export.Export(ctx, build)
		if err != nil {
			return err
		}

		if !ok {
			continue
		}

		commandParts = append(commandParts, fmt.Sprintf("'source' '%v'", dir))
	}

	appDir := filepath.Join(build.BuildDir, AppDir)
	envDir := filepath.Join(build.BuildDir, EnvironmentDir)

	// compile
	commandParts = append(commandParts, fmt.Sprintf("'%v' '%v' '%v' '%v'", compile, appDir, build.CacheDir, envDir))

	compileCmd := exec.CommandContext(ctx, "bash", "-c", strings.Join(commandParts, ";")) // #nosec G204
	compileCmd.Env = environ(build)
	compileCmd.Stderr, compileCmd.Stdout = build.Stdout, build.Stderr
	if err := compileCmd.Run(); err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}

	return nil
}

func (b *Buildpack) Export(ctx context.Context, build *Build) (string, bool, error) {
	export := filepath.Join(build.BuildDir, BuildpacksDir, b.Directory, "export")

	if _, err := os.Stat(export); err == nil {
		return export, true, nil
	} else if os.IsNotExist(err) {
		return "", false, nil
	} else {
		return "", false, err
	}
}
