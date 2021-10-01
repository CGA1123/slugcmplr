package slugcmplr

import (
	"context"
	"fmt"
	"sort"

	heroku "github.com/heroku/heroku-go/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// MetadataCmd wraps up all the information required to fetch metadata require
// for compilation of application.
type MetadataCmd struct {
	Heroku      *heroku.Service
	Application string
	SourceDir   string
	BuildDir    string
	Tracer      trace.Tracer
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
func (m *MetadataCmd) Execute(ctx context.Context, _ Outputter) (*MetadataResult, error) {
	sctx, span := m.Tracer.Start(ctx, "slugcmplr_metadata",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("type", "command"),
			attribute.String("command.name", "metadata"),
		),
	)
	defer span.End()

	g, gctx := errgroup.WithContext(sctx)

	var commit string
	var app *heroku.App
	var config heroku.ConfigVarInfoForAppResult
	var buildpacks heroku.BuildpackInstallationListResult

	commit, err := Commit(m.SourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve commit: %w", err)
	}

	g.Go(func() error {
		a, err := m.Heroku.AppInfo(gctx, m.Application)
		if err != nil {
			return fmt.Errorf("failed to fetch app info: %w", err)
		}

		app = a

		return nil
	})

	g.Go(func() error {
		c, err := m.Heroku.ConfigVarInfoForApp(gctx, m.Application)
		if err != nil {
			return fmt.Errorf("failed to fetch app configuration: %w", err)
		}

		config = c

		return nil
	})

	g.Go(func() error {
		bpi, err := m.Heroku.BuildpackInstallationList(gctx, m.Application, nil)
		if err != nil {
			return fmt.Errorf("failed to fetch app buildpacks: %w", err)
		}

		buildpacks = bpi

		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &MetadataResult{
		ApplicationName: app.Name,
		Stack:           app.Stack.Name,
		Buildpacks:      buildBuildpacks(buildpacks),
		ConfigVars:      buildConfigVars(config),
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
