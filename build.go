package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cga1123/slugcmplr/procfile"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

const envVar = "SLUGCMPLR"

var buildCmd = &cobra.Command{
	Use:   "build [application]",
	Short: "Triggers a build of your application.",
	Long: `The build command will create a clone of your target application and
create a standard Heroku build. The build will _not_ run the release task in
your Procfile if it is defined.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := netrcClient()
		if err != nil {
			wrn(os.Stderr, "error creating client from .netrc: %v", err)

			return err
		}

		commit, err := commit()
		if err != nil {
			wrn(os.Stderr, "error detecting HEAD commit: %v", err)

			return err
		}

		step(os.Stdout, "Compiling %v via %v for commit %v", args[0], compileAppID, commit[:7])

		exist, err := procfileExist()
		if err != nil {
			wrn(os.Stderr, "error detecting procfile: %v", err)

			return fmt.Errorf("error detecting procfile: %v", err)
		}

		if exist {
			log(os.Stdout, "Procfile detected. Escaping release task...")
			if err := escapeReleaseTask("./Procfile"); err != nil {
				wrn(os.Stderr, "error escaping release task: %v", err)

				return fmt.Errorf("error escaping release task: %v", err)
			}
		} else {
			log(os.Stdout, "No Procfile detected")
		}

		// Tar it up
		sha := sha256.New()
		archive := &bytes.Buffer{}

		step(os.Stdout, "Creating source code tarball...")
		if err := targz(".", tar.FormatGNU, archive, sha); err != nil {
			wrn(os.Stderr, "error creating tarball: %v", err)

			return fmt.Errorf("error creating tarball: %v", err)
		}

		checksum := "SHA256:" + hex.EncodeToString(sha.Sum(nil))

		log(os.Stdout, "Checksum: %v", checksum)
		log(os.Stdout, "Size: %v", archive.Len())

		step(os.Stdout, "Uploading source code tarball...")
		src, err := upload(context.Background(), client, archive)
		if err != nil {
			return err
		}

		dbg(os.Stdout, "Get URL: %v", src.SourceBlob.GetURL)
		dbg(os.Stdout, "Put URL: %v", src.SourceBlob.PutURL)

		step(os.Stdout, "Synchronising %v to %v...", args[0], compileAppID)
		if err := synchronise(context.Background(), client, args[0], compileAppID); err != nil {
			wrn(os.Stderr, "error synchronising applications: %v", err)

			return err
		}

		step(os.Stdout, "Creating compilation build...")
		build, err := client.BuildCreate(context.Background(), compileAppID, heroku.BuildCreateOpts{
			SourceBlob: struct {
				Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
				URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
				Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
			}{
				Checksum: heroku.String(checksum),
				URL:      heroku.String(src.SourceBlob.GetURL),
				Version:  heroku.String(commit)}})
		if err != nil {
			wrn(os.Stderr, "error triggering build: %v", err)

			return err
		}

		if err := outputStream(os.Stdout, build.OutputStreamURL); err != nil {
			wrn(os.Stdout, "error streaming build output: %v", err)

			return fmt.Errorf("error streaming build output: %v", err)
		}

		if verbose {
			build, err := client.BuildInfo(context.Background(), build.App.ID, build.ID)
			if err != nil {
				dbg(os.Stdout, "error fetching build metadata for debug: %v", err)
			} else {
				dbg(os.Stdout, "build ID: %v", build.ID)
				dbg(os.Stdout, "slug: %v", build.Slug)
				dbg(os.Stdout, "release: %v", build.Release)
			}
		}

		return nil
	},
}

func init() {
	buildCmd.Flags().StringVar(&compileAppID, "compiler", "", "The Heroku application to compile on (required)")
	buildCmd.MarkFlagRequired("compiler")

	rootCmd.AddCommand(buildCmd)
}

func procfileExist() (bool, error) {
	if fi, err := os.Stat("./Procfile"); err == nil {
		return fi.Mode().IsRegular(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}

// escapeReleaseTask rewrites the Procfile at the given path, adding
// a short-circuit operator to the release task if EnvVar is set.
//
// This ensures that the release task is a noop when compiling slugs.
func escapeReleaseTask(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0755)
	if err != nil {
		return fmt.Errorf("error opening Procfile: %w", err)
	}

	procf, err := procfile.Read(f)
	if err != nil {
		return err
	}

	cmd, ok := procf.Entrypoint("release")

	// no release command specified, no need to escape it!
	if !ok {
		return f.Close()
	}

	procf.Add("release", fmt.Sprintf("[ ! -z $%v ] || %v", envVar, cmd))

	// Truncate the Procfile
	if err := f.Truncate(0); err != nil {
		return err
	}

	// Seek to the begining of the file
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}

	// Write the updated Procfile
	if _, err := procf.Write(f); err != nil {
		return err
	}

	return f.Close()
}

// targz will walk srcDirPath recursively and write the correspoding G-Zipped Tar
// Archive to the given writers.
func targz(srcDirPath string, format tar.Format, writers ...io.Writer) error {
	if _, err := os.Stat(srcDirPath); err != nil {
		return fmt.Errorf("source directory does not exist: %w", err)
	}

	mw := io.MultiWriter(writers...)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	return filepath.WalkDir(srcDirPath, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("file moved or removed while building tarball: %w", err)
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		header, err := tar.FileInfoHeader(info, d.Name())
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(strings.TrimPrefix(file, srcDirPath), string(filepath.Separator))
		header.Format = format

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		f.Close()

		return nil
	})
}

func synchronise(ctx context.Context, h *heroku.Service, target, compile string) error {
	bpi, err := h.BuildpackInstallationList(ctx, target, nil)
	if err != nil {
		return err
	}
	sort.Slice(bpi, func(a, b int) bool {
		return bpi[a].Ordinal < bpi[b].Ordinal
	})
	buildpacks := make([]struct {
		Buildpack string `json:"buildpack" url:"buildpack,key"`
	}, len(bpi))
	for i, bp := range bpi {
		buildpacks[i] = struct {
			Buildpack string `json:"buildpack" url:"buildpack,key"`
		}{Buildpack: bp.Buildpack.URL}
	}

	configuration, err := h.ConfigVarInfoForApp(ctx, target)
	if err != nil {
		return err
	}
	configuration[envVar] = heroku.String("true")

	if _, err := h.AppUpdate(ctx, compile, heroku.AppUpdateOpts{Maintenance: heroku.Bool(true)}); err != nil {
		return fmt.Errorf("error setting maintenance mode: %w", err)
	}

	if _, err := h.ConfigVarUpdate(ctx, compile, configuration); err != nil {
		return err
	}

	if _, err := h.BuildpackInstallationUpdate(ctx, compile, heroku.BuildpackInstallationUpdateOpts{Updates: buildpacks}); err != nil {
		return err
	}

	return nil
}

func upload(ctx context.Context, h *heroku.Service, blob *bytes.Buffer) (*heroku.Source, error) {
	src, err := h.SourceCreate(ctx)
	if err != nil {
		return nil, fmt.Errorf("error creating source: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, src.SourceBlob.PutURL, blob)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	var body string
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	body = string(b)

	if response.StatusCode > 399 {
		return nil, fmt.Errorf("HTTP %v: %v", response.Status, body)
	}

	return src, nil
}
