package slugcmplr

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/slugignore"
	"github.com/otiai10/copy"
)

// PrepareCmd wraps up all the information required to prepare an application
// for slug compilation.
type PrepareCmd struct {
	SourceDir  string
	BuildDir   string
	ConfigVars map[string]string
	Buildpacks []*BuildpackReference
}

// PrepareResult contains the result of preparing, including the path to the
// base build directory and metadata about the downloaded buildpacks -- their
// order and paths.
type PrepareResult struct {
	BuildDir   string
	Buildpacks []*buildpack.Buildpack
}

// Execute prepares an application for compilation by download all required
// buildpack, copying the source into the build directory, and writing out the
// application environment.
func (p *PrepareCmd) Execute(ctx context.Context, _ Outputter) (*PrepareResult, error) {
	envDir := filepath.Join(p.BuildDir, buildpack.EnvironmentDir)
	buildpacksDir := filepath.Join(p.BuildDir, buildpack.BuildpacksDir)
	appDir := filepath.Join(p.BuildDir, buildpack.AppDir)

	// Download Buildpacks
	// TODO: Do this in parallel?
	bps := make([]*buildpack.Buildpack, len(p.Buildpacks))
	for i, bp := range p.Buildpacks {
		src, err := buildpack.ParseSource(bp.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse buildpack source: %w", err)
		}

		bp, err := src.Download(ctx, buildpacksDir)
		if err != nil {
			return nil, fmt.Errorf("failed to download buildpack: %w", err)
		}

		bps[i] = bp
	}

	// Write config vars to the envDir.
	if err := os.MkdirAll(envDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to mkdir (%v): %w", envDir, err)
	}

	for name, value := range p.ConfigVars {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte(value), 0600); err != nil {
			return nil, fmt.Errorf("error writing %v: %w", name, err)
		}
	}

	// Copy source to the SourceDir, respecting any .slugignore file, if it
	// exists.
	ignore, err := slugignore.ForDirectory(p.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read .slugignore: %w", err)
	}

	if err := copy.Copy(p.SourceDir, appDir, copy.Options{
		Skip: func(_ fs.FileInfo, path, _ string) (bool, error) {
			return ignore.IsIgnored(
				strings.TrimPrefix(path, p.SourceDir),
			), nil
		},
	}); err != nil {
		return nil, fmt.Errorf("failed to copy source: %w", err)
	}

	return &PrepareResult{
		BuildDir:   p.BuildDir,
		Buildpacks: bps,
	}, nil
}
