package buildpack

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
)

// Source described the interface for a buildpack source.
type Source interface {
	Download(ctx context.Context, baseDir string) (*Buildpack, error)
	Dir() string
}

// ParseSource parses and returns an appropriate Source for the given buildpack URL.
//
// It currently supports official buildpack URLS of form
// `urn:buildpack:foo/bar` and GitHub repository URLs.
//
// All other URLs are defaulted to being a generic URL to some GZipped Tar
// archive.
//
// It does not support arbitrary .git URLs.
func ParseSource(url string) (Source, error) {
	// official buildpack
	if strings.HasPrefix(url, "urn:buildpack:") {
		return &TargzSource{
			RawURL: url,
			URL:    fmt.Sprintf("https://buildpack-registry.s3.amazonaws.com/buildpacks/%v.tgz", strings.TrimPrefix(url, "urn:buildpack:")),
			github: false,
		}, nil
	}

	// github buildpack
	if strings.HasPrefix(url, "https://github.com/") {
		parts := strings.SplitN(url, "#", 2)
		repo := strings.TrimSuffix(parts[0], ".git")

		var ref string
		if len(parts) != 2 || parts[1] == "" {
			ref = "HEAD"
		} else {
			ref = parts[1]
		}

		return &TargzSource{URL: fmt.Sprintf("%v/tarball/%v", repo, ref), RawURL: url, github: true}, nil
	}

	// TODO: support non github .git URLs (clone)
	return &TargzSource{RawURL: url, URL: url, github: false}, nil
}

// TargzSource implements Source for GZipped Tar buildpack archives.
//
// It has custom support for GitHub hosted repositories and their nested
// tarball format.
type TargzSource struct {
	RawURL string
	URL    string
	github bool
}

// Dir returns the directory name for the buildpack being downloaded.
func (s *TargzSource) Dir() string {
	return sum256(s.URL)
}

// Download downloads a GZipped Tar buildpack from the configured URL into Dir()
// relative to baseDir.
func (s *TargzSource) Download(ctx context.Context, baseDir string) (*Buildpack, error) {
	path, err := download(ctx, s.URL)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close() // nolint:errcheck

	if err := Untargz(
		f,
		filepath.Join(baseDir, s.Dir()),
		s.github,
	); err != nil {
		return nil, fmt.Errorf("failed to untar: %v", err)
	}

	return &Buildpack{Directory: s.Dir(), URL: s.RawURL}, nil
}

func download(ctx context.Context, url string) (string, error) {
	path := ""
	attempt := func() error {
		f, err := os.CreateTemp(os.TempDir(), "")
		if err != nil {
			return fmt.Errorf("failed creating tmpfile: %w", err)
		}
		defer f.Close() // nolint:errcheck

		path = f.Name()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("failed to build request: %w", err)
		}

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to make request: %w", err)
		}

		if res.StatusCode > 299 {
			return fmt.Errorf("non 2XX response code: %v", res.StatusCode)
		}
		defer res.Body.Close() // nolint:errcheck

		if _, err := io.Copy(f, res.Body); err != nil {
			return fmt.Errorf("error downloading file contents: %w", err)
		}

		if err := f.Close(); err != nil {
			return fmt.Errorf("error flushing file: %w", err)
		}

		return nil
	}
	if err := backoff.RetryNotify(attempt, backoffConfig(), backoffNotify); err != nil {
		return "", fmt.Errorf("error uploading slug: %w", err)
	}

	return path, nil
}

func backoffNotify(err error, retryIn time.Duration) {
	log.Printf("Error downloading buildpack in %s: %s", retryIn, err)
}

func backoffConfig() backoff.BackOff {
	return &backoff.ExponentialBackOff{
		InitialInterval:     100 * time.Millisecond,
		RandomizationFactor: 0.25,
		Multiplier:          2.0,
		MaxInterval:         5 * time.Second,
		MaxElapsedTime:      5 * time.Minute,
		Clock:               backoff.SystemClock,
	}
}

// Untargz extracts a GZipped Tarball into dir.
//
// If unnest is true, during extraction the root directory of every filepath
// being extracted is skipped.
//
// TODO: should this accept a context so we can cancel?
func Untargz(r io.Reader, dir string, unnest bool) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("failed to build gzip reader: %w", err)
	}
	defer gz.Close() // nolint:errcheck

	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to mkdir (%v): %w", dir, err)
	}

	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}

	tarball := tar.NewReader(gz)
	symlinks := []*tar.Header{}

	for {
		header, err := tarball.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return fmt.Errorf("error extracting tar archive: %w", err)
		}

		if header == nil {
			continue
		}

		path := buildFilepath(dir, unnest, header)

		// Malicious tar files can have entries containing multiple ".." in
		// their path, which could lead to writing files outside of the
		// expected directory (e.g. overriding a common executable)
		//
		// Mitigate this by ensuring that the fully resolved path  is still
		// within the expected baseDir. (filepath.Join calls filepath.Clean to
		// clean the resulting path, evaluating any `..`)
		//
		// See: https://snyk.io/research/zip-slip-vulnerability
		if !strings.HasPrefix(path, dir) {
			return fmt.Errorf("detected zipslip processing: %v (fullpath=%v)", header.Name, path)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return err
			}

			for {
				_, err := io.CopyN(f, tarball, 1024)
				if err != nil {
					if err == io.EOF {
						break
					}
					return err
				}
			}

			if err := f.Close(); err != nil {
				return fmt.Errorf("failed to close written file (%v): %w", path, err)
			}
		case tar.TypeSymlink:
			symlinks = append(symlinks, header)
		}
	}

	for _, header := range symlinks {
		path := buildFilepath(dir, unnest, header)

		evalPath, err := filepath.EvalSymlinks(filepath.Join(path, "..", header.Linkname)) // #nosec G305
		if err != nil {
			return fmt.Errorf("failed to evaluate symlink: %w", err)
		}

		if !strings.HasPrefix(evalPath, dir) {
			return fmt.Errorf("symlink breaks out of path")
		}

		if err := os.Symlink(header.Linkname, path); err != nil {
			return fmt.Errorf("failed to symlink %v -> %v: %w", header.Linkname, path, err)
		}
	}

	return nil
}

func buildFilepath(basePath string, unnest bool, header *tar.Header) string {
	var path string
	if unnest {
		parts := strings.SplitN(header.Name, string(filepath.Separator), 2)
		if len(parts) > 1 {
			path = filepath.Join(basePath, parts[1])
		} else {
			path = filepath.Join(basePath, parts[0])
		}
	} else {
		path = filepath.Join(basePath, header.Name) // #nosec G305
	}

	return path
}

func sum256(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}
