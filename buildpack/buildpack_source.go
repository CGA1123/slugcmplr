package buildpack

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Source interface {
	Download(ctx context.Context, baseDir string) (*Buildpack, error)
	Dir() string
}

type targzSource struct {
	RawURL string
	URL    string
	github bool
}

func (s *targzSource) Dir() string {
	return sum256(s.URL)
}

func (s *targzSource) Download(ctx context.Context, baseDir string) (*Buildpack, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	if res.StatusCode > 299 {
		return nil, fmt.Errorf("non 2XX response code: %v", res.StatusCode)
	}
	defer res.Body.Close() // nolint:errcheck

	gz, err := gzip.NewReader(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to build gzip reader: %w", err)
	}
	defer gz.Close() // nolint:errcheck

	basePath := filepath.Join(baseDir, s.Dir())
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to mkdir (%v): %w", basePath, err)
	}

	basePath, err = filepath.EvalSymlinks(basePath)
	if err != nil {
		return nil, err
	}

	tarball := tar.NewReader(gz)
	symlinks := []*tar.Header{}

	for {
		header, err := tarball.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("error extracting tar archive: %w", err)
		}

		if header == nil {
			continue
		}

		path := buildFilepath(basePath, s, header)

		// Malicious tar files can have entries containing multiple ".." in
		// their path, which could lead to writing files outside of the
		// expected directory (e.g. overriding a common executable)
		//
		// Mitigate this by ensuring that the fully resolved path  is still
		// within the expected baseDir. (filepath.Join calls filepath.Clean to
		// clean the resulting path, evaluating any `..`)
		//
		// See: https://snyk.io/research/zip-slip-vulnerability
		if !strings.HasPrefix(path, basePath) {
			return nil, fmt.Errorf("detected zipslip processing: %v (fullpath=%v)", header.Name, path)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, header.FileInfo().Mode()); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return nil, err
			}

			for {
				_, err := io.CopyN(f, tarball, 1024)
				if err != nil {
					if err == io.EOF {
						break
					}
					return nil, err
				}
			}

			if err := f.Close(); err != nil {
				return nil, fmt.Errorf("failed to close written file (%v): %w", path, err)
			}
		case tar.TypeSymlink:
			symlinks = append(symlinks, header)
		}
	}

	for _, header := range symlinks {
		path := buildFilepath(basePath, s, header)

		evalPath, err := filepath.EvalSymlinks(filepath.Join(path, "..", header.Linkname)) // #nosec G305
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate symlink: %w", err)
		}

		if !strings.HasPrefix(evalPath, basePath) {
			return nil, fmt.Errorf("symlink breaks out of path")
		}

		if err := os.Symlink(header.Linkname, path); err != nil {
			return nil, fmt.Errorf("failed to symlink %v -> %v: %w", header.Linkname, path, err)
		}
	}

	return &Buildpack{Directory: s.Dir(), URL: s.RawURL}, nil
}

func buildFilepath(basePath string, s *targzSource, header *tar.Header) string {
	var path string
	if s.github {
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

func ParseSource(url string) (Source, error) {
	// official buildpack
	if strings.HasPrefix(url, "urn:buildpack:") {
		return &targzSource{
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

		return &targzSource{URL: fmt.Sprintf("%v/tarball/%v", repo, ref), RawURL: url, github: true}, nil
	}

	// TODO: support non github .git URLs (clone)
	return &targzSource{RawURL: url, URL: url, github: false}, nil
}

func sum256(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}
