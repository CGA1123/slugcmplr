package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/procfile"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

type Compile struct {
	Application   string                 `json:"application"`
	Stack         string                 `json:"stack"`
	SourceVersion string                 `json:"source_version"`
	Buildpacks    []*buildpack.Buildpack `json:"buildpacks"`
}

func compile(ctx context.Context, h *heroku.Service, buildDir, cacheDir string) error {
	step(os.Stdout, "Reading metadata")
	log(os.Stdout, "From: %v", filepath.Join(buildDir, "meta.json"))

	m, err := os.Open(filepath.Join(buildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}
	defer m.Close()

	c := &Compile{}
	if err := json.NewDecoder(m).Decode(c); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	log(os.Stdout, "application: %v", c.Application)
	log(os.Stdout, "stack: %v", c.Stack)
	log(os.Stdout, "buildpacks: %v", len(c.Buildpacks))

	build := &buildpack.Build{
		CacheDir:      cacheDir,
		BuildDir:      buildDir,
		Stack:         c.Stack,
		SourceVersion: c.SourceVersion,
	}

	previousBuildpacks := make([]*buildpack.Buildpack, 0, len(c.Buildpacks))
	var detectedBuildpack string

	// run buildpacks
	for _, bp := range c.Buildpacks {
		detected, ok, err := bp.Detect(ctx, build)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}

		step(os.Stdout, "%v app detected", detected)

		if err := bp.Compile(ctx, previousBuildpacks, build); err != nil {
			return err
		}

		step(os.Stdout, "Build succeeded!")

		previousBuildpacks = append(previousBuildpacks, bp)
	}

	appDir := filepath.Join(buildDir, buildpack.AppDir)

	// tar up
	tarball, err := targz(appDir, filepath.Join(buildDir, "app.tgz"))
	if err != nil {
		return fmt.Errorf("error creating tarball: %v", err)
	}

	f, err := os.Open(filepath.Join(appDir, "Procfile"))
	if err != nil {
		return err
	}
	defer f.Close()

	p, err := procfile.Read(f)
	if err != nil {
		return err
	}

	// create a slug
	slug, err := h.SlugCreate(ctx, c.Application, heroku.SlugCreateOpts{
		Checksum:                     heroku.String(tarball.checksum),
		Commit:                       heroku.String(c.SourceVersion),
		Stack:                        heroku.String(c.Stack),
		BuildpackProvidedDescription: heroku.String(detectedBuildpack),
		ProcessTypes:                 p,
	})
	if err != nil {
		return err
	}

	if err := upload(ctx, strings.ToUpper(slug.Blob.Method), slug.Blob.URL, tarball.path); err != nil {
		return err
	}

	fmt.Printf("created slug %v\n", slug.ID)

	return nil
}

func compileCmd() *cobra.Command {
	var cacheDir, buildDir string

	cmd := &cobra.Command{
		Use:   "compile",
		Short: "compile the target applications",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := netrcClient()
			if err != nil {
				return err
			}

			if cacheDir == "" {
				cd, err := os.MkdirTemp("", "")
				if err != nil {
					return err
				}

				cacheDir = cd
			}

			dbg(os.Stdout, "buildDir: %v", buildDir)
			dbg(os.Stdout, "cacheDir: %v", cacheDir)

			return compile(cmd.Context(), client, buildDir, cacheDir)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir")

	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "The cache directory")

	return cmd
}
