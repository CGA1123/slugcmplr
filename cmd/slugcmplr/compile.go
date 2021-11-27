package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/buildpack"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

// Compile contains the configuration required for the compile subcommand.
type Compile struct {
	Application   string                 `json:"application"`
	Stack         string                 `json:"stack"`
	SourceVersion string                 `json:"source_version"`
	Buildpacks    []*buildpack.Buildpack `json:"buildpacks"`
}

func compile(ctx context.Context, out outputter, h *heroku.Service, c *Compile, buildDir, cacheDir string) error {
	log(out, "application: %v", c.Application)
	log(out, "stack: %v", c.Stack)
	log(out, "buildpacks: %v", len(c.Buildpacks))

	compileCmd := &slugcmplr.CompileCmd{
		CacheDir:      cacheDir,
		BuildDir:      buildDir,
		Stack:         c.Stack,
		SourceVersion: c.SourceVersion,
		Buildpacks:    c.Buildpacks,
	}

	result, err := compileCmd.Execute(ctx, out)
	if err != nil {
		return fmt.Errorf("error during compilation: %w", err)
	}

	uploadCmd := &slugcmplr.UploadCmd{
		Heroku:            h,
		Application:       c.Application,
		Checksum:          result.SlugChecksum,
		Path:              result.SlugPath,
		DetectedBuildpack: result.DetectedBuildpack,
		SourceVersion:     result.SourceVersion,
		Stack:             result.Stack,
		ProcessTypes:      result.Procfile,
	}

	u, err := uploadCmd.Execute(ctx, out)
	if err != nil {
		return fmt.Errorf("error when uploading slug: %w", err)
	}

	fmt.Printf("created slug %v\n", u.SlugID)

	step(out, "Writing metadata")
	log(out, "To: %v", filepath.Join(buildDir, "release.json"))
	b := &bytes.Buffer{}
	if err := json.NewEncoder(b).Encode(
		&release{Application: c.Application, Slug: u.SlugID, Commit: u.SourceVersion},
	); err != nil {
		return fmt.Errorf("error encoding metadata: %w", err)
	}

	// #nosec G306
	if err := os.WriteFile(
		filepath.Join(buildDir, "release.json"),
		b.Bytes(),
		0644,
	); err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}

	return nil
}

func compileCmd(verbose bool) *cobra.Command {
	var cacheDir, buildDir string

	cmd := &cobra.Command{
		Use:   "compile",
		Short: "compile the target applications",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := outputterFromCmd(cmd, verbose)

			if cacheDir == "" {
				cd, err := os.MkdirTemp("", "")
				if err != nil {
					return err
				}

				cacheDir = cd
			}

			dbg(output, "buildDir: %v", buildDir)
			dbg(output, "cacheDir: %v", cacheDir)

			step(output, "Reading metadata")
			log(output, "From: %v", filepath.Join(buildDir, "meta.json"))

			m, err := os.Open(filepath.Join(buildDir, "meta.json"))
			if err != nil {
				return fmt.Errorf("failed to read metadata: %w", err)
			}
			defer m.Close() // nolint:errcheck

			c := &Compile{}
			if err := json.NewDecoder(m).Decode(c); err != nil {
				return fmt.Errorf("failed to decode metadata: %w", err)
			}

			client, err := netrcClient(output)
			if err != nil {
				return err
			}

			return compile(cmd.Context(), output, client, c, buildDir, cacheDir)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "The cache directory")

	return cmd
}
