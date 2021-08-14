package buildpack

import (
	"context"
	"fmt"
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
}

type Buildpack struct {
	Directory string `json:"directory"`
}

func (b *Buildpack) Run(ctx context.Context, previousBuildpacks []*Buildpack, build *Build) (string, bool, error) {
	detected, ok, err := b.Detect(ctx, build)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}

	if err := b.Compile(ctx, previousBuildpacks, build); err != nil {
		return "", false, err
	}

	return detected, true, nil
}

func (b *Buildpack) Detect(ctx context.Context, build *Build) (string, bool, error) {
	detect := filepath.Join(b.Directory, "bin", "detect")
	stdout := &strings.Builder{}

	detectCmd := exec.CommandContext(ctx, detect, filepath.Join(build.BuildDir, AppDir))
	detectCmd.Stderr, detectCmd.Stdout = os.Stderr, stdout

	if err := detectCmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", false, nil
		}

		return "", false, err
	}

	return strings.TrimSpace(stdout.String()), true, nil
}

func (b *Buildpack) Compile(ctx context.Context, exports []*Buildpack, build *Build) error {
	compile := filepath.Join(b.Directory, "bin", "compile")
	commandParts := []string{}

	// exports
	for _, export := range exports {
		dir, ok, err := export.Export(ctx)
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

	compileCmd := exec.CommandContext(ctx, "bash", "-c", strings.Join(commandParts, ";"))
	compileCmd.Stderr, compileCmd.Stdout = os.Stderr, os.Stdout
	if err := compileCmd.Run(); err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}

	return nil
}

func (b *Buildpack) Export(ctx context.Context) (string, bool, error) {
	export := filepath.Join(b.Directory, "export")

	if _, err := os.Stat(export); err == nil {
		return export, true, nil
	} else if os.IsNotExist(err) {
		return "", false, nil
	} else {
		return "", false, err
	}
}
