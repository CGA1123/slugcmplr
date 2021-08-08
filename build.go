package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cga1123/slugcmplr/procfile"
	heroku "github.com/heroku/heroku-go/v5"
)

const envVar = "SLUGCMPLR"

func build(ctx context.Context, production, compile, commit string, client *heroku.Service) error {
	step(os.Stdout, "Compiling %v via %v for commit %v", production, compile, commit[:7])

	if err := escapeReleaseTask(); err != nil {
		wrn(os.Stderr, "error escaping release task: %v", err)

		return fmt.Errorf("error escaping release task: %v", err)
	}

	step(os.Stdout, "Creating source code tarball...")
	srcDirPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("error getting current directory: %w", err)
	}

	tarball, err := targz(srcDirPath)
	if err != nil {
		wrn(os.Stderr, "error creating tarball: %v", err)

		return fmt.Errorf("error creating tarball: %v", err)
	}

	log(os.Stdout, "Checksum: %v", tarball.checksum)
	log(os.Stdout, "Size: %v", tarball.blob.Len())

	step(os.Stdout, "Uploading source code tarball...")
	src, err := client.SourceCreate(ctx)
	if err != nil {
		return fmt.Errorf("error creating source: %w", err)
	}

	if err := upload(ctx, http.MethodPut, src.SourceBlob.PutURL, tarball.blob); err != nil {
		return err
	}

	step(os.Stdout, "Synchronising %v to %v...", production, compile)
	if err := synchronise(ctx, client, production, compile); err != nil {
		wrn(os.Stderr, "error synchronising applications: %v", err)

		return err
	}

	step(os.Stdout, "Creating compilation build...")
	build, err := client.BuildCreate(ctx, compile, heroku.BuildCreateOpts{
		SourceBlob: struct {
			Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
			URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
			Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
		}{
			Checksum: heroku.String(tarball.checksum),
			URL:      heroku.String(src.SourceBlob.GetURL),
			Version:  heroku.String(commit)}})
	if err != nil {
		wrn(os.Stderr, "error triggering build: %v", err)

		return err
	}

	dbg(os.Stdout, "output stream: %v", build.OutputStreamURL)
	dbg(os.Stdout, "source URL: %v", build.SourceBlob.URL)
	dbg(os.Stdout, "source Checksum: %v", ptrStr(build.SourceBlob.Checksum))
	dbg(os.Stdout, "source Version: %v", ptrStr(build.SourceBlob.Version))

	if err := outputStream(os.Stdout, build.OutputStreamURL); err != nil {
		wrn(os.Stdout, "error streaming build output: %v", err)

		return fmt.Errorf("error streaming build output: %v", err)
	}

	step(os.Stdout, "Verifying build status...")

	for i := 0; i < 5; i++ {
		build, err := client.BuildInfo(ctx, compile, build.ID)
		if err != nil {
			wrn(os.Stderr, "error checking build state: %v", err)

			return err
		}

		switch status := build.Status; status {
		case "pending":
			log(os.Stderr, "build is still pending...")
			time.Sleep(5 * time.Second)
		case "failed":
			wrn(os.Stderr, "build failed, try again?")

			return fmt.Errorf("build failed, try again?")
		case "succeeded":
			log(os.Stdout, "build succeeded")
			dbg(os.Stdout, "ID: %v", build.ID)
			dbg(os.Stdout, "slug: %v", build.Slug)
			dbg(os.Stdout, "release: %v", build.Release)

			return nil
		default:
			wrn(os.Stderr, "unknown release status: %v", status)

			return fmt.Errorf("unknown release status returned by Heroku: %v", status)
		}
	}

	wrn(os.Stderr, "build is still pending, aborting checks.")

	return fmt.Errorf("build is still pending after a while")
}

func escapeReleaseTask() error {
	f, err := os.OpenFile("./Procfile", os.O_RDWR, 0755)
	if err != nil {
		// Procfile doesn't exists, that's ok :)
		if errors.Is(err, os.ErrNotExist) {
			log(os.Stdout, "No Procfile detected")

			return nil
		} else {
			return fmt.Errorf("error opening Procfile: %w", err)
		}
	}

	procf, err := procfile.Read(f)
	if err != nil {
		return err
	}

	cmd, ok := procf.Entrypoint("release")
	if !ok {
		log(os.Stdout, "No release task specified")
		return f.Close()
	}

	log(os.Stdout, "Escaping release task: %v", cmd)
	procf.Add("release", fmt.Sprintf("[ ! -z $%v ] || %v", envVar, cmd))

	log(os.Stdout, "Writing Procfile")
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

type tarball struct {
	blob     *bytes.Buffer
	checksum string
}

// targz will walk srcDirPath recursively and write the correspoding G-Zipped Tar
// Archive to the given writers.
func targz(srcDirPath string) (*tarball, error) {
	sha, archive := sha256.New(), &bytes.Buffer{}
	mw := io.MultiWriter(sha, archive)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	walk := func(file string, d fs.DirEntry, err error) error {
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

		// Heroku requires GNU Tar format (at least for slugs, maybe not for build sources?)
		//
		// https://devcenter.heroku.com/articles/platform-api-deploying-slugs#create-slug-archive
		header.Format = tar.FormatGNU

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		return f.Close()
	}

	if err := filepath.WalkDir(srcDirPath, walk); err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	// explicitly close to ensure we flush to archive and sha, make sure we get
	// a correct checksum.
	if err := tw.Close(); err != nil {
		return nil, err
	}

	if err := gzw.Close(); err != nil {
		return nil, err
	}

	return &tarball{blob: archive, checksum: fmt.Sprintf("SHA256:%v", hex.EncodeToString(sha.Sum(nil)))}, nil
}

// TODO: Should the dyno formation be explicitly set to 0?
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

	targetAppFeatures, err := fetchAppFeatures(ctx, h, target)
	if err != nil {
		return err
	}

	compileAppFeatures, err := fetchAppFeatures(ctx, h, compile)
	if err != nil {
		return err
	}

	for k, v := range targetAppFeatures {
		if compileAppFeatures[k] != v {
			_, err := h.AppFeatureUpdate(ctx, compile, k, heroku.AppFeatureUpdateOpts{Enabled: v})
			if err != nil {
				return fmt.Errorf("updating compile app features: %w", err)
			}
		}
	}

	return nil
}

func fetchAppFeatures(ctx context.Context, h *heroku.Service, app string) (map[string]bool, error) {
	features, err := h.AppFeatureList(ctx, app, nil)
	if err != nil {
		return nil, err
	}

	featMap := make(map[string]bool, len(features))

	for _, feat := range features {
		featMap[feat.ID] = feat.Enabled
	}

	return featMap, nil
}

func upload(ctx context.Context, method, url string, blob *bytes.Buffer) error {
	dbg(os.Stdout, "uploading: %v %v", method, url)

	req, err := http.NewRequestWithContext(ctx, method, url, blob)
	if err != nil {
		return err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var body string
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	body = string(b)

	if response.StatusCode > 399 {
		return fmt.Errorf("HTTP %v: %v", response.Status, body)
	}

	return nil
}
