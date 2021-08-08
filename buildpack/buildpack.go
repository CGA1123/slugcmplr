package buildpack

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Build struct {
	BuildDir      string
	EnvDir        string
	CacheDir      string
	Stack         string
	SourceVersion string
}

type Buildpack interface {
	Detect(context.Context, *Build) (string, bool, error)
	Compile(context.Context, []Buildpack, *Build) error
	Export(context.Context) (string, bool, error)
	Run(context.Context, []Buildpack, *Build) (string, bool, error)
}

type buildpack struct {
	directory string
}

func (b *buildpack) Run(ctx context.Context, previousBuildpacks []Buildpack, build *Build) (string, bool, error) {
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

func (b *buildpack) Detect(ctx context.Context, build *Build) (string, bool, error) {
	detect := filepath.Join(b.directory, "bin", "detect")
	stdout := &strings.Builder{}

	detectCmd := exec.CommandContext(ctx, detect, build.BuildDir)
	detectCmd.Stderr, detectCmd.Stdout = os.Stderr, stdout

	if err := detectCmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return "", false, nil
		}

		return "", false, err
	}

	return strings.TrimSpace(stdout.String()), true, nil
}

func (b *buildpack) Compile(ctx context.Context, exports []Buildpack, build *Build) error {
	compile := filepath.Join(b.directory, "bin", "compile")
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

	// compile
	commandParts = append(commandParts, fmt.Sprintf("'%v' '%v' '%v' '%v'", compile, build.BuildDir, build.CacheDir, build.EnvDir))

	compileCmd := exec.CommandContext(ctx, "bash", "-c", strings.Join(commandParts, ";"))
	compileCmd.Stderr, compileCmd.Stdout = os.Stderr, os.Stdout
	if err := compileCmd.Run(); err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}

	return nil
}

func (b *buildpack) Export(ctx context.Context) (string, bool, error) {
	export := filepath.Join(b.directory, "export")

	if _, err := os.Stat(export); err == nil {
		return export, true, nil
	} else if os.IsNotExist(err) {
		return "", false, nil
	} else {
		return "", false, err
	}
}
