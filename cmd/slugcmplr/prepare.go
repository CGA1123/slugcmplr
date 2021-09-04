package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/slugignore"
	"github.com/otiai10/copy"
	"github.com/spf13/cobra"
)

type BuildpackDescription struct {
	Name string
	URL  string
}

type Prepare struct {
	ApplicationName string
	Stack           string
	Buildpacks      []*BuildpackDescription
	ConfigVars      map[string]string
	SourceDir       string
	BuildDir        string
}

// nolint: godot
// prepare
//
// - copy source dir       // DONE
// - run slugcleanup       // TODO - do it as part of the copy? using Skip option
// - download buildpacks   // DONE
// - download config vars  // DONE
// - dump metadata file    // DONE
func prepare(ctx context.Context, out outputter, p *Prepare) error {
	envDir := filepath.Join(p.BuildDir, buildpack.EnvironmentDir)
	buildpacksDir := filepath.Join(p.BuildDir, buildpack.BuildpacksDir)
	appDir := filepath.Join(p.BuildDir, buildpack.AppDir)

	// metadata
	commit, err := commitDir(out, p.SourceDir)
	if err != nil {
		return fmt.Errorf("failed to resolve HEAD commit: %w", err)
	}

	step(out, "Preparing app: %v", p.ApplicationName)
	log(out, "stack: %v", p.Stack)
	log(out, "%v config vars", len(p.ConfigVars))
	log(out, "%v buildpacks", len(p.Buildpacks))
	log(out, "commit: %v", commit)
	log(out, "build directory: %v", p.BuildDir)
	log(out, "source directory: %v", p.SourceDir)

	// download buildpacks
	bps := make([]*buildpack.Buildpack, len(p.Buildpacks))
	for i, bp := range p.Buildpacks {
		step(out, "Downloading buildpack: %v", bp.URL)
		src, err := buildpack.ParseSource(bp.URL)
		if err != nil {
			return fmt.Errorf("failed to parse buildpack source: %w", err)
		}

		log(out, "Output: %v", src.Dir())

		bp, err := src.Download(ctx, buildpacksDir)
		if err != nil {
			return fmt.Errorf("failed to download buildpack: %w", err)
		}

		bps[i] = bp
	}

	step(out, "Using buildpacks:")
	for i, bp := range p.Buildpacks {
		log(out, "%v. %v", i+1, bp.Name)
	}

	step(out, "Writing configuration variables")
	log(out, "Output: %v", envDir)

	// write env
	if err := os.MkdirAll(envDir, 0700); err != nil {
		return fmt.Errorf("failed to mkdir (%v): %w", envDir, err)
	}

	for name, value := range p.ConfigVars {
		dbg(out, "writing env: %v", name)

		log(out, "%v: %v bytes", name, len(value))

		if err := os.WriteFile(filepath.Join(envDir, name), []byte(value), 0600); err != nil {
			return fmt.Errorf("error writing %v: %w", name, err)
		}
	}

	step(out, "Copying source")
	log(out, "From: %v", p.SourceDir)
	log(out, "To: %v", appDir)

	ignore, err := slugignore.ForDirectory(p.SourceDir)
	if err != nil {
		return fmt.Errorf("failed to read .slugignore: %v", err)
	}

	// copy source
	if err := copy.Copy(p.SourceDir, appDir, copy.Options{
		Skip: func(path string) (bool, error) {
			return ignore.IsIgnored(
				strings.TrimPrefix(path, p.SourceDir),
			), nil
		},
	}); err != nil {
		return fmt.Errorf("failed to copy source: %w", err)
	}

	step(out, "Writing metadata")
	log(out, "To: %v", filepath.Join(p.BuildDir, "meta.json"))
	f, err := os.Create(filepath.Join(p.BuildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}
	defer f.Close() // nolint:errcheck

	c := &Compile{
		Application:   p.ApplicationName,
		Stack:         p.Stack,
		SourceVersion: commit,
		Buildpacks:    bps,
	}

	if err := json.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("error dumping metadata: %w", err)
	}

	return f.Close()
}

func prepareCmd(verbose bool) *cobra.Command {
	var buildDir, srcDir string

	cmd := &cobra.Command{
		Use:   "prepare [target]",
		Short: "prepare the target application for compilation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			application := args[0]
			output := outputterFromCmd(cmd, verbose)
			h, err := netrcClient(output)
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

			dbg(output, "buildDir: %v", buildDir)
			dbg(output, "srcDir: %v", srcDir)
			dbg(output, "application: %v", application)

			ctx := cmd.Context()

			app, err := h.AppInfo(ctx, application)
			if err != nil {
				return err
			}

			configuration, err := h.ConfigVarInfoForApp(ctx, application)
			if err != nil {
				return err
			}

			configVars := make(map[string]string, len(configuration))
			for k, v := range configuration {
				if v == nil {
					continue
				}

				configVars[k] = *v
			}

			bpi, err := h.BuildpackInstallationList(ctx, application, nil)
			if err != nil {
				return err
			}
			sort.Slice(bpi, func(a, b int) bool {
				return bpi[a].Ordinal < bpi[b].Ordinal
			})

			buildpacks := make([]*BuildpackDescription, len(bpi))
			for i, bp := range bpi {
				buildpacks[i] = &BuildpackDescription{
					Name: bp.Buildpack.Name,
					URL:  bp.Buildpack.URL,
				}
			}

			return prepare(ctx, output, &Prepare{
				ApplicationName: application,
				Stack:           app.Stack.Name,
				ConfigVars:      configVars,
				Buildpacks:      buildpacks,
				SourceDir:       srcDir,
				BuildDir:        buildDir,
			})
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.Flags().StringVar(&srcDir, "source-dir", "", "The source app directory")

	return cmd
}
