package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/buildpack"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

const defaultImage = "ghcr.io/cga1123/slugcmplr:" + slugcmplr.StackReplacePattern

// Compile contains the configuration required for the compile subcommand.
type Compile struct {
	Application   string                 `json:"application"`
	Stack         string                 `json:"stack"`
	SourceVersion string                 `json:"source_version"`
	Buildpacks    []*buildpack.Buildpack `json:"buildpacks"`
}

func bootstrapDocker(ctx context.Context, out outputter, buildDir, cacheDir, netrc, image string) error {
	step(out, "Reading metadata")
	log(out, "From: %v", filepath.Join(buildDir, "meta.json"))

	m, err := os.Open(filepath.Join(buildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}
	defer m.Close() // nolint:errcheck

	c := &Compile{}
	if err := json.NewDecoder(m).Decode(c); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	imageName := slugcmplr.StackImage(image, c.Stack)

	log(out, "Using: %v", imageName)

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

	dbg(out, "dockerRun: %v", dockerRun.String())

	dockerRun.Stderr, dockerRun.Stdout = out.ErrOrStderr(), out.OutOrStdout()

	return dockerRun.Run()
}

func compile(ctx context.Context, out outputter, h *heroku.Service, buildDir, cacheDir string) error {
	step(out, "Reading metadata")
	log(out, "From: %v", filepath.Join(buildDir, "meta.json"))

	m, err := os.Open(filepath.Join(buildDir, "meta.json"))
	if err != nil {
		return fmt.Errorf("failed to read metadata: %w", err)
	}
	defer m.Close() // nolint:errcheck

	c := &Compile{}
	if err := json.NewDecoder(m).Decode(c); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

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

	r := &release{Application: c.Application, Slug: u.SlugID, Commit: u.SourceVersion}

	step(out, "Writing metadata")
	log(out, "To: %v", filepath.Join(buildDir, "release.json"))
	f, err := os.Create(filepath.Join(buildDir, "release.json"))
	if err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}
	defer f.Close() // nolint:errcheck

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
	cmd.Flags().StringVar(
		&image,
		"image",
		defaultImage,
		fmt.Sprintf(
			"Override docker image to use, include %s in order to substitute the stack name",
			slugcmplr.StackReplacePattern,
		),
	)

	return cmd
}
