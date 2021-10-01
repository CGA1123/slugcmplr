package slugcmplr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/slugignore"
	"github.com/otiai10/copy"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// PrepareCmd wraps up all the information required to prepare an application
// for slug compilation.
type PrepareCmd struct {
	SourceDir  string
	BuildDir   string
	ConfigVars map[string]string
	Buildpacks []*BuildpackReference
	Tracer     trace.Tracer
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
	sctx, span := p.Tracer.Start(ctx, "slugcmplr_prepare",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("type", "command"),
			attribute.String("command.name", "prepare"),
		),
	)
	defer span.End()

	g, gctx := errgroup.WithContext(sctx)

	var bps []*buildpack.Buildpack
	g.Go(func() error {
		return p.writeEnvDir(gctx)
	})

	g.Go(func() error {
		return p.copySource(gctx)
	})

	g.Go(func() error {
		var err error

		bps, err = p.downloadBuildpacks(gctx)

		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return &PrepareResult{
		BuildDir:   p.BuildDir,
		Buildpacks: bps,
	}, nil
}

func (p *PrepareCmd) copySource(ctx context.Context) error {
	_, span := p.Tracer.Start(ctx, "copy_source",
		trace.WithSpanKind(trace.SpanKindInternal),
	)
	defer span.End()

	appDir := filepath.Join(p.BuildDir, buildpack.AppDir)
	ignore, err := slugignore.ForDirectory(p.SourceDir)
	if err != nil {
		return fmt.Errorf("failed to read .slugignore: %w", err)
	}

	if err := copy.Copy(p.SourceDir, appDir, copy.Options{
		Skip: func(path string) (bool, error) {
			return ignore.IsIgnored(
				strings.TrimPrefix(path, p.SourceDir),
			), nil
		},
	}); err != nil {
		return fmt.Errorf("failed to copy source: %w", err)
	}

	return nil
}

func (p *PrepareCmd) writeEnvDir(ctx context.Context) error {
	_, span := p.Tracer.Start(ctx, "write_env_dir",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Int("env.count", len(p.ConfigVars)),
		),
	)
	defer span.End()

	envDir := filepath.Join(p.BuildDir, buildpack.EnvironmentDir)
	if err := os.MkdirAll(envDir, 0700); err != nil {
		return fmt.Errorf("failed to mkdir (%v): %w", envDir, err)
	}

	for name, value := range p.ConfigVars {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte(value), 0600); err != nil {
			return fmt.Errorf("error writing %v: %w", name, err)
		}
	}

	return nil
}

func (p *PrepareCmd) downloadBuildpacks(ctx context.Context) ([]*buildpack.Buildpack, error) {
	sctx, span := p.Tracer.Start(ctx, "download_buildpacks",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.Int("buildpacks.count", len(p.Buildpacks)),
		),
	)
	defer span.End()

	buildpacksDir := filepath.Join(p.BuildDir, buildpack.BuildpacksDir)

	bps := make([]*buildpack.Buildpack, len(p.Buildpacks))
	download := func(ctx context.Context, urn string, idx int) func() error {
		return func() error {
			sctx, span := p.Tracer.Start(ctx, "download_buildpack",
				trace.WithSpanKind(trace.SpanKindInternal),
				trace.WithAttributes(
					attribute.String("buildpack.urn", urn),
					attribute.Int("buildpack.idx", idx),
				),
			)
			defer span.End()

			src, err := buildpack.ParseSource(urn)
			if err != nil {
				return fmt.Errorf("failed to parse buildpack source: %w", err)
			}

			bp, err := src.Download(sctx, buildpacksDir)
			if err != nil {
				return fmt.Errorf("failed to download buildpack: %w", err)
			}

			bps[idx] = bp

			return nil
		}
	}

	g, gctx := errgroup.WithContext(sctx)
	for i, bp := range p.Buildpacks {
		g.Go(download(gctx, bp.URL, i))
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return bps, nil
}
