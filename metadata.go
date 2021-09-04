package slugcmplr

import (
	"context"
	"fmt"
	"sort"

	heroku "github.com/heroku/heroku-go/v5"
)

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
