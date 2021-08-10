package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/procfile"
	heroku "github.com/heroku/heroku-go/v5"
)

// TODO:
// slugcmplr compile [APP]
//   --source-dir [DIR] // default to current directory
//   --cache-dir [DIR] // default to tmpdir/cache
//   --env-dir [DIR] // default to tmpdir/env
//   --buildpacks-dir [DIR] //defaule to tmpdir/buildpacks
//   --output [DIR] // default to tmpdir
//
// slugcmplr upload [APP] --slug [PATH]
//
// slugcmplr release [APP] --slug-id [ID]
//
// How to make it easy to get slug id from
func compile(ctx context.Context, production, commit string, h *heroku.Service) error {
	baseDir, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("failed to make temp dir: %w", err)
	}

	dbg(os.Stdout, "baseDir: %s", baseDir)

	// TODO: The cache directory should be passed in, allowing users to have
	// arbitrary caching strategies
	cacheDir := filepath.Join(baseDir, "cache")
	envDir := fmt.Sprintf("%s/%s", baseDir, "environment")

	appDir, err := os.Getwd() // TODO: should this be an arg?
	if err != nil {
		return err
	}

	buildpacksDir := fmt.Sprintf("%s/%s", baseDir, "buildpacks")

	app, err := h.AppInfo(ctx, production)
	if err != nil {
		return err
	}

	// Fetch config vars
	configuration, err := h.ConfigVarInfoForApp(ctx, production)
	if err != nil {
		return err
	}
	for k, v := range configuration {
		dbg(os.Stdout, "config (%v) = %v", k, *v)
	}

	// Fetch buildpacks
	dbg(os.Stdout, "fetching buildpacks")
	bpi, err := h.BuildpackInstallationList(ctx, production, nil)
	if err != nil {
		return err
	}
	sort.Slice(bpi, func(a, b int) bool {
		return bpi[a].Ordinal < bpi[b].Ordinal
	})

	bps := make([]buildpack.Buildpack, len(bpi))
	for i, bp := range bpi {
		dbg(os.Stdout, "buildpack (%v) = %v (%v)", i, bp.Buildpack.Name, bp.Buildpack.URL)

		src, err := buildpack.ParseSource(bp.Buildpack.URL)
		if err != nil {
			return err
		}

		bp, err := src.Download(ctx, buildpacksDir)
		if err != nil {
			return err
		}

		bps[i] = bp
	}

	// write environment
	if err := dumpEnv(envDir, configuration); err != nil {
		return err
	}

	dbg(os.Stdout, "dumped env")

	build := &buildpack.Build{
		BuildDir:      appDir,
		EnvDir:        envDir,
		CacheDir:      cacheDir,
		Stack:         app.Stack.Name,
		SourceVersion: commit,
	}

	previousBuildpacks := make([]buildpack.Buildpack, 0, len(bps))
	var detectedBuildpack string

	dbg(os.Stdout, "%v buildpacks detected", len(bps))
	// run buildpacks
	for i, bp := range bps {
		dbg(os.Stdout, "running buildpack: %v", i)
		detected, ok, err := bp.Run(ctx, previousBuildpacks, build)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		detectedBuildpack = detected

		previousBuildpacks = append(previousBuildpacks, bp)
	}

	// tar up
	tarball, err := targz(appDir)
	if err != nil {
		return fmt.Errorf("error creating tarball: %v", err)
	}

	f, err := os.Open(filepath.Join(appDir, "Procfile"))
	if err != nil {
		return err
	}

	p, err := procfile.Read(f)
	if err != nil {
		return err
	}

	// create a slug
	slug, err := h.SlugCreate(ctx, production, heroku.SlugCreateOpts{
		Checksum:                     heroku.String(tarball.checksum),
		Commit:                       heroku.String(commit),
		Stack:                        heroku.String(app.Stack.Name),
		BuildpackProvidedDescription: heroku.String(detectedBuildpack),
		ProcessTypes:                 p,
	})
	if err != nil {
		return err
	}

	if err := upload(ctx, strings.ToUpper(slug.Blob.Method), slug.Blob.URL, tarball.blob); err != nil {
		return err
	}

	fmt.Printf("created slug %v\n", slug.ID)

	return nil
}

func dumpEnv(dir string, env map[string]*string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to mkdir (%v): %w", dir, err)
	}

	for name, value := range env {
		dbg(os.Stdout, "writing env: %v", name)

		if value == nil {
			continue
		}

		if err := os.WriteFile(filepath.Join(dir, name), []byte(*value), 0600); err != nil {
			return fmt.Errorf("error writing %v: %w", name, err)
		}
	}

	return nil
}
