package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/procfile"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

const DefaultImage = "ghcr.io/cga1123/slugcmplr:%stack%"

type Compile struct {
	Application   string                 `json:"application"`
	Stack         string                 `json:"stack"`
	SourceVersion string                 `json:"source_version"`
	Buildpacks    []*buildpack.Buildpack `json:"buildpacks"`
}

func bootstrapDocker(ctx context.Context, cmd Outputter, buildDir, cacheDir, netrc, image string) error {
	step(cmd, "Reading metadata")
	log(cmd, "From: %v", filepath.Join(buildDir, "meta.json"))

	m, err := os.Open(filepath.Join(buildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}
	defer m.Close()

	c := &Compile{}
	if err := json.NewDecoder(m).Decode(c); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	imageName := strings.ReplaceAll(image, "%stack%", c.Stack)

	log(cmd, "Using: %v", imageName)

	dockerRun := exec.CommandContext(ctx, "docker", "run",
		"--volume", fmt.Sprintf("%v:/tmp/build", buildDir),
		"--volume", fmt.Sprintf("%v:/tmp/cache", cacheDir),
		"--volume", fmt.Sprintf("%v:/tmp/netrc", netrc),
		"--env", "NETRC=/tmp/netrc",
		imageName,
		"compile",
		"--local",
		"--build-dir", "/tmp/build",
		"--cache-dir", "/tmp/cache",
	) // #nosec G204

	dbg(cmd, "dockerRun: %v", dockerRun.String())

	dockerRun.Stderr, dockerRun.Stdout = cmd.ErrOrStderr(), cmd.OutOrStdout()

	return dockerRun.Run()
}

func compile(ctx context.Context, cmd Outputter, h *heroku.Service, buildDir, cacheDir string) error {
	step(cmd, "Reading metadata")
	log(cmd, "From: %v", filepath.Join(buildDir, "meta.json"))

	m, err := os.Open(filepath.Join(buildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}
	defer m.Close()

	c := &Compile{}
	if err := json.NewDecoder(m).Decode(c); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	log(cmd, "application: %v", c.Application)
	log(cmd, "stack: %v", c.Stack)
	log(cmd, "buildpacks: %v", len(c.Buildpacks))

	build := &buildpack.Build{
		CacheDir:      cacheDir,
		BuildDir:      buildDir,
		Stack:         c.Stack,
		SourceVersion: c.SourceVersion,
		Stdout:        cmd.OutOrStdout(),
		Stderr:        cmd.ErrOrStderr(),
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
			step(cmd, "App not compatible with buildpack: %v", bp.URL)
			wrn(cmd, "Compilation failed")

			return fmt.Errorf("buildpack detection failure")
		}

		step(cmd, "%v app detected", detected)

		if err := bp.Compile(ctx, previousBuildpacks, build); err != nil {
			return err
		}

		previousBuildpacks = append(previousBuildpacks, bp)
	}

	appDir := filepath.Join(buildDir, buildpack.AppDir)

	// read Procfile
	step(cmd, "Discovering process types")

	pf, err := os.Open(filepath.Join(appDir, "Procfile"))
	if err != nil {
		return err
	}
	defer pf.Close()

	p, err := procfile.Read(pf)
	if err != nil {
		return err
	}

	// tar up
	step(cmd, "Compressing...")

	tarball, err := targz(appDir, filepath.Join(buildDir, "app.tgz"))
	if err != nil {
		return fmt.Errorf("error creating tarball: %v", err)
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

	r := &Release{
		Application: c.Application,
		Slug:        slug.ID,
		Commit:      c.SourceVersion,
	}

	step(cmd, "Writing metadata")
	log(cmd, "To: %v", filepath.Join(buildDir, "release.json"))
	f, err := os.Create(filepath.Join(buildDir, "release.json"))
	if err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(r); err != nil {
		return fmt.Errorf("error dumping metadata: %w", err)
	}

	return f.Close()
}

func compileCmd(verbose bool) *cobra.Command {
	var cacheDir, buildDir, image string
	var local bool

	cmd := &cobra.Command{
		Use:   "compile",
		Short: "compile the target applications",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			output := OutputterFromCmd(cmd, verbose)

			if cacheDir == "" {
				cd, err := os.MkdirTemp("", "")
				if err != nil {
					return err
				}

				cacheDir = cd
			}

			dbg(output, "buildDir: %v", buildDir)
			dbg(output, "cacheDir: %v", cacheDir)

			if local {
				client, err := netrcClient(output)
				if err != nil {
					return err
				}

				return compile(cmd.Context(), output, client, buildDir, cacheDir)
			}

			netrcpath, err := netrcPath()
			if err != nil {
				return fmt.Errorf("failed to find netrc path: %w", err)
			}

			return bootstrapDocker(cmd.Context(), output, buildDir, cacheDir, netrcpath, image)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	cmd.Flags().BoolVar(&local, "local", false, "Run compilation locally")
	cmd.Flags().StringVar(&cacheDir, "cache-dir", "", "The cache directory")
	cmd.Flags().StringVar(&image, "image", DefaultImage, "Override docker image to use, include %stack% in order to substitute the stack name")

	return cmd
}
