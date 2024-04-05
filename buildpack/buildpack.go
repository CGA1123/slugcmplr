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
	// AppDir is the path relative to the build directory where the source to
	// be built is stored.
	AppDir = "app"

	// EnvironmentDir is the path relative to the build directory where the
	// environment/config variables are stored.
	EnvironmentDir = "environment"

	// BuildpacksDir is the path relative to the build directory where
	// buildpacks are stored and downloaded to.
	BuildpacksDir = "buildpacks"
)

// Build contains the information required to run a series of buildpacks.
type Build struct {
	BuildDir      string
	CacheDir      string
	Stack         string
	SourceVersion string
	Stdout        io.Writer
	Stderr        io.Writer
}

// Buildpack describes a buildpack that has been downloaded to the local
// filesystem.
type Buildpack struct {
	URL       string `json:"url"`
	Directory string `json:"directory"`
}

func environ(b *Build) []string {
	return append(os.Environ(), "STACK="+b.Stack, "SOURCE_VERSION="+b.SourceVersion)
}

// Detect determines whether the buildpack can be applied to the current
// application.
//
// It is expected that slug compilation fail if any configured buildpack cannot
// be applied to the current application.
//
// See: https://devcenter.heroku.com/articles/buildpack-api#bin-detect
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

// Compile applies the buildpack to the current application.
//
// It calls Export() on all previous buildpacks, to ensure any required setup
// is propagated between buildpacks.
//
// See: https://devcenter.heroku.com/articles/buildpack-api#bin-compile
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
	compileCmd.Dir = filepath.Join(build.BuildDir, BuildpacksDir, b.Directory)
	compileCmd.Stderr, compileCmd.Stdout = build.Stdout, build.Stderr
	if err := compileCmd.Run(); err != nil {
		return fmt.Errorf("failed to compile: %w", err)
	}

	return nil
}

// Export returns the path to the export file for the given buildpack, to be
// sourced by subsequent buildpacks, if it exists.
//
// See: https://devcenter.heroku.com/articles/buildpack-api#composing-multiple-buildpacks
func (b *Buildpack) Export(_ context.Context, build *Build) (string, bool, error) {
	export := filepath.Join(build.BuildDir, BuildpacksDir, b.Directory, "export")

	if _, err := os.Stat(export); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}

	return export, true, nil
}
