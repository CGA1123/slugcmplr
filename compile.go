package slugcmplr

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/processfile"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CompileCmd wraps up all the information required to compile the contents of
// SourceDir into a deployable artifact/slug.
type CompileCmd struct {
	CacheDir      string
	BuildDir      string
	Stack         string
	SourceVersion string
	Buildpacks    []*buildpack.Buildpack
	Tracer        trace.Tracer
}

// CompileResult contains metadata about the result of Executing CompileCmd.
type CompileResult struct {
	SlugPath          string
	SlugChecksum      string
	SourceVersion     string
	Procfile          processfile.Procfile
	DetectedBuildpack string
	Stack             string
}

// Execute applies the buildpacks to the SourceDir in their specific order,
// before compressing the result of these applications into a GZipped Tar file
// within BuildDir.
func (c *CompileCmd) Execute(ctx context.Context, out Outputter) (*CompileResult, error) {
	sctx, span := c.Tracer.Start(ctx, "slugcmplr_compile",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("type", "command"),
			attribute.String("command.name", "compile"),
		),
	)
	defer span.End()

	build := &buildpack.Build{
		CacheDir:      c.CacheDir,
		BuildDir:      c.BuildDir,
		Stack:         c.Stack,
		SourceVersion: c.SourceVersion,
		Stdout:        out.OutOrStdout(),
		Stderr:        out.ErrOrStderr(),
	}

	detectedBuildpack := ""
	previousBuildpacks := make([]*buildpack.Buildpack, 0, len(c.Buildpacks))

	for _, bp := range c.Buildpacks {
		detected, err := c.compile(sctx, build, bp, previousBuildpacks)
		if err != nil {
			return nil, err
		}

		previousBuildpacks = append(previousBuildpacks, bp)
		detectedBuildpack = detected
	}

	pf, err := os.Open(filepath.Join(c.BuildDir, buildpack.AppDir, "Procfile"))
	if err != nil {
		return nil, fmt.Errorf("error opening Procfile: %w", err)
	}
	defer pf.Close() // nolint:errcheck

	procfile, err := processfile.Read(pf)
	if err != nil {
		return nil, err
	}

	tarball, err := Targz(
		filepath.Join(c.BuildDir, buildpack.AppDir),
		filepath.Join(c.BuildDir, "app.tgz"),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating tarball: %w", err)
	}

	return &CompileResult{
		Procfile:          procfile,
		DetectedBuildpack: detectedBuildpack,
		SlugPath:          tarball.Path,
		SlugChecksum:      tarball.Checksum,
		SourceVersion:     c.SourceVersion,
		Stack:             c.Stack,
	}, nil
}

func (c *CompileCmd) compile(ctx context.Context, build *buildpack.Build, bp *buildpack.Buildpack, previousBuildpacks []*buildpack.Buildpack) (string, error) {
	sctx, span := c.Tracer.Start(ctx, "slugcmplr_compile_buildpack",
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			attribute.String("buildpack.url", bp.URL),
			attribute.String("build.stack", build.Stack),
			attribute.String("build.source_version", build.SourceVersion),
		),
	)
	defer span.End()

	detected, ok, err := bp.Detect(sctx, build)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("buildpack detection failure: %v", bp.URL)
	}

	if err := bp.Compile(sctx, previousBuildpacks, build); err != nil {
		return "", err
	}

	return detected, nil
}
