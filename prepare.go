package slugcmplr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/slugignore"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/otiai10/copy"
)

// BuildpackReference is a reference to a buildpack, containing its raw URL and
// Name.
type BuildpackReference struct {
	Name string
	URL  string
}

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
func (p *PrepareCmd) Execute(ctx context.Context, out Outputter) (*PrepareResult, error) {
	envDir := filepath.Join(p.BuildDir, buildpack.EnvironmentDir)
	buildpacksDir := filepath.Join(p.BuildDir, buildpack.BuildpacksDir)
	appDir := filepath.Join(p.BuildDir, buildpack.AppDir)

	// Download Buildpacks
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
		return nil, fmt.Errorf("failed to read .slugignore: %v", err)
	}

	if err := copy.Copy(p.SourceDir, appDir, copy.Options{
		Skip: func(path string) (bool, error) {
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

// MetadataCmd wraps up all the information required to fetch metadata require
// for compilation of application.
type MetadataCmd struct {
	Heroku      *heroku.Service
	Application string
	SourceDir   string
	BuildDir    string
}

// MetadataResult contains the result of resolving metadata for an application.
type MetadataResult struct {
	ApplicationName string
	Stack           string
	Buildpacks      []*BuildpackReference
	ConfigVars      map[string]string
	SourceVersion   string
}

// Execute fetches the applications name, stack, buildpacks, and config
// variables. It will also resolve the current HEAD commit of the source to be
// compiled.
func (m *MetadataCmd) Execute(ctx context.Context, out Outputter) (*MetadataResult, error) {
	commit, err := Commit(m.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve commit: %w", err)
	}

	app, err := m.Heroku.AppInfo(ctx, m.Application)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app info: %w", err)
	}

	conf, err := m.Heroku.ConfigVarInfoForApp(ctx, m.Application)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app configuration: %w", err)
	}

	bpi, err := m.Heroku.BuildpackInstallationList(ctx, m.Application, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch app buildpacks: %w", err)
	}

	return &MetadataResult{
		ApplicationName: app.Name,
		Stack:           app.Stack.Name,
		Buildpacks:      buildBuildpacks(bpi),
		ConfigVars:      buildConfigVars(conf),
		SourceVersion:   commit,
	}, nil
}

func buildConfigVars(conf heroku.ConfigVarInfoForAppResult) map[string]string {
	configVars := make(map[string]string, len(conf))
	for k, v := range conf {
		if v == nil {
			continue
		}

		configVars[k] = *v
	}

	return configVars
}

func buildBuildpacks(bpi heroku.BuildpackInstallationListResult) []*BuildpackReference {
	sort.Slice(bpi, func(a, b int) bool {
		return bpi[a].Ordinal < bpi[b].Ordinal
	})

	buildpacks := make([]*BuildpackReference, len(bpi))
	for i, bp := range bpi {
		buildpacks[i] = &BuildpackReference{
			Name: bp.Buildpack.Name,
			URL:  bp.Buildpack.URL,
		}
	}

	return buildpacks
}
