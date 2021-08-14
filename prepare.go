package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/cga1123/slugcmplr/buildpack"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/otiai10/copy"
	"github.com/spf13/cobra"
)

type Prepare struct {
	Application string
	SourceDir   string
	BuildDir    string
}

// prepare
//
// - copy source dir       // DONE
// - run slugcleanup       // TODO - do it as part of the copy?
// - download buildpacks   // DONE
// - download config vars  // DONE
// - dump metadata file    // DONE
func prepare(ctx context.Context, h *heroku.Service, p *Prepare) error {
	envDir := filepath.Join(p.BuildDir, buildpack.EnvironmentDir)
	buildpacksDir := filepath.Join(p.BuildDir, buildpack.BuildpacksDir)
	appDir := filepath.Join(p.BuildDir, buildpack.AppDir)

	// dump metadata
	commit, err := commitDir(p.SourceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve HEAD commit: %w", err)
	}

	app, err := h.AppInfo(ctx, p.Application)
	if err != nil {
		return err
	}

	configuration, err := h.ConfigVarInfoForApp(ctx, p.Application)
	if err != nil {
		return err
	}

	bpi, err := h.BuildpackInstallationList(ctx, p.Application, nil)
	if err != nil {
		return err
	}

	step(os.Stdout, "Preparing app: %v", p.Application)
	log(os.Stdout, "id: %v", app.ID)
	log(os.Stdout, "stack: %v", app.Stack.Name)
	log(os.Stdout, "%v config vars", len(configuration))
	log(os.Stdout, "%v buildpacks", len(bpi))
	log(os.Stdout, "commit: %v", commit)
	log(os.Stdout, "build directory: %v", p.BuildDir)
	log(os.Stdout, "source directory: %v", p.SourceDir)

	// download buildpacks
	sort.Slice(bpi, func(a, b int) bool {
		return bpi[a].Ordinal < bpi[b].Ordinal
	})

	bps := make([]*buildpack.Buildpack, len(bpi))
	for i, bp := range bpi {
		step(os.Stdout, "Downloading buildpack: %v", bp.Buildpack.Name)
		src, err := buildpack.ParseSource(bp.Buildpack.URL)
		if err != nil {
			return fmt.Errorf("failed to parse buildpack source: %w", err)
		}

		log(os.Stdout, "Output: %v", src.Dir())

		bp, err := src.Download(ctx, buildpacksDir)
		if err != nil {
			return fmt.Errorf("failed to download buildpack: %w", err)
		}

		bps[i] = bp
	}

	step(os.Stdout, "Using buildpacks:")
	for i, bp := range bpi {
		log(os.Stdout, "%v. %v", i+1, bp.Buildpack.Name)
	}

	step(os.Stdout, "Writing configuration variables")
	log(os.Stdout, "Output: %v", envDir)

	// write env
	if err := os.MkdirAll(envDir, 0700); err != nil {
		return fmt.Errorf("failed to mkdir (%v): %w", envDir, err)
	}

	for name, value := range configuration {
		dbg(os.Stdout, "writing env: %v", name)

		if value == nil {
			continue
		}

		log(os.Stdout, "%v: %v bytes", name, len(*value))

		if err := os.WriteFile(filepath.Join(envDir, name), []byte(*value), 0600); err != nil {
			return fmt.Errorf("error writing %v: %w", name, err)
		}
	}

	step(os.Stdout, "Copying source")
	log(os.Stdout, "From: %v", p.SourceDir)
	log(os.Stdout, "To: %v", appDir)

	// copy source
	if err := copy.Copy(p.SourceDir, appDir); err != nil {
		return fmt.Errorf("failed to copy source: %w", err)
	}

	step(os.Stdout, "Writing metadata")
	log(os.Stdout, "To: %v", filepath.Join(p.BuildDir, "meta.json"))
	f, err := os.Create(filepath.Join(p.BuildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}
	defer f.Close()

	c := &Compile{
		Application:   p.Application,
		Stack:         app.Stack.Name,
		SourceVersion: commit,
		Buildpacks:    bps,
	}

	if err := json.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("error dumping metadata: %w", err)
	}

	return f.Close()
}

func prepareCmd() *cobra.Command {
	var buildDir, srcDir string

	cmd := &cobra.Command{
		Use:   "prepare [target]",
		Short: "prepare the target application for compilation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			application := args[0]
			client, err := netrcClient()
			if err != nil {
				return err
			}

			if buildDir == "" {
				bd, err := os.MkdirTemp("", "")
				if err != nil {
					return err
				}

				buildDir = bd
			}

			if srcDir == "" {
				sd, err := os.Getwd()
				if err != nil {
					return err
				}

				srcDir = sd
			}

			dbg(os.Stdout, "buildDir: %v", buildDir)
			dbg(os.Stdout, "srcDir: %v", srcDir)
			dbg(os.Stdout, "application: %v", application)

			return prepare(cmd.Context(), client, &Prepare{
				Application: application,
				SourceDir:   srcDir,
				BuildDir:    buildDir,
			})
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.Flags().StringVar(&srcDir, "source-dir", "", "The source app directory")

	return cmd
}
