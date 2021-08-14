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
	BuildDir      string                 `json:"build_dir"`
	Buildpacks    []*buildpack.Buildpack `json:"buildpacks"`
}

func compile(ctx context.Context, h *heroku.Service, buildDir, cacheDir string) error {
	m, err := os.Open(filepath.Join(buildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}
	defer m.Close()

	var c *Compile
	if err := json.NewDecoder(m).Decode(c); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	build := &buildpack.Build{
		CacheDir:      cacheDir,
		BuildDir:      c.BuildDir,
		Stack:         c.Stack,
		SourceVersion: c.SourceVersion,
	}

	previousBuildpacks := make([]*buildpack.Buildpack, 0, len(c.Buildpacks))
	var detectedBuildpack string

	dbg(os.Stdout, "%v buildpacks detected", len(c.Buildpacks))

	// run buildpacks
	for i, bp := range c.Buildpacks {
		dbg(os.Stdout, "running buildpack: %v", i)
		detected, ok, err := bp.Run(ctx, previousBuildpacks, build)
		if err != nil {
			return err
		}
		if !ok {
			// should we fail if detect fails? i think heroku does this!
			continue
		}

		detectedBuildpack = detected

		previousBuildpacks = append(previousBuildpacks, bp)
	}

	appDir := filepath.Join(c.BuildDir, buildpack.AppDir)

	// tar up
	tarball, err := targz(appDir)
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

	if err := upload(ctx, strings.ToUpper(slug.Blob.Method), slug.Blob.URL, tarball.blob); err != nil {
		return err
	}

	fmt.Printf("created slug %v\n", slug.ID)

	return nil
}

func compileCmd() *cobra.Command {
	var cacheDir string

	cmd := &cobra.Command{
		Use:   "compile [target]",
		Short: "compile the target applications",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			buildDir := args[0]
			client, err := netrcClient()
			if err != nil {
				return err
			}

			dbg(os.Stdout, "buildDir: %v", buildDir)

			if cacheDir == "" {
				cd, err := os.MkdirTemp("", "")
				if err != nil {
					return err
				}

				cacheDir = cd
			}

			return compile(cmd.Context(), client, buildDir, cacheDir)
		},
	}

	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "The cache directory")
	cmd.MarkFlagRequired("cache-dir")

	return cmd
}
