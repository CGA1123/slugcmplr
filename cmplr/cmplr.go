package cmplr

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bgentry/go-netrc/netrc"
	"github.com/bissyio/slugcmplr/procfile"

	heroku "github.com/heroku/heroku-go/v5"
)

const (
	EnvVar = "SLUGCMPLR"
)

// Heroku returns a new *heroku.Service that will authenticate using credentials
// in the .netrc file.
//
// The .netrc file is automatically populated by the Heroku CLI after running
// `heroku login`.
func Client() (*heroku.Service, error) {
	rcfile, err := loadNetrc()
	if err != nil {
		return nil, err
	}

	machine := rcfile.FindMachine("api.heroku.com")
	if machine == nil {
		return nil, fmt.Errorf("no .netrc entry for api.heroku.com found")
	}

	return heroku.NewService(&http.Client{
		Transport: &heroku.Transport{
			Username: machine.Login,
			Password: machine.Password}}), nil
}

func loadNetrc() (*netrc.Netrc, error) {
	if fromEnv := os.Getenv("NETRC"); fromEnv != "" {
		return netrc.ParseFile(fromEnv)
	}

	u, err := user.Current()
	if err != nil {
		return nil, err
	}

	return netrc.ParseFile(filepath.Join(u.HomeDir, ".netrc"))
}

type SourceBlob struct {
	Checksum, URL, Version string
}

// CreateBuild will create and trigger a build on the CompileApplication for the SourceBlob,
// the buildpacks used for the build are extracted from the SourceApplication
func CreateBuild(ctx context.Context, h *heroku.Service, compile string, tar *SourceBlob) (*heroku.Build, error) {
	return h.BuildCreate(ctx, compile, heroku.BuildCreateOpts{
		SourceBlob: struct {
			Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
			URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
			Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
		}{
			Checksum: heroku.String(tar.Checksum),
			URL:      heroku.String(tar.URL),
			Version:  heroku.String(tar.Version)}})
}

type BlobURL struct {
	Get, Put string
}

func Upload(ctx context.Context, h *heroku.Service, blob *bytes.Buffer) (*BlobURL, error) {
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

	return &BlobURL{Get: src.SourceBlob.GetURL, Put: src.SourceBlob.PutURL}, nil
}

// EscapeProcfileReleaseTask rewrites the Procfile at the given path, adding
// a short-circuit operator to the release task if EnvVar is set.
//
// This ensures that the release task is a noop when compiling slugs.
func EscapeReleaseTask(path string) error {
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

	procf.Add("release", fmt.Sprintf("[ ! -z $%v ] || %v", EnvVar, cmd))

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

// Tar will walk srcDirPath recursively and write the correspoding G-Zipped Tar
// Archive to the given writers.
func Tar(srcDirPath string, format tar.Format, writers ...io.Writer) error {
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

func Synchronise(ctx context.Context, h *heroku.Service, target, compile string) error {
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
	configuration[EnvVar] = heroku.String("true")

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
