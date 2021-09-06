package slugcmplr

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cga1123/slugcmplr/buildpack"
	"github.com/cga1123/slugcmplr/services/jobs"
	"github.com/cga1123/slugcmplr/slugignore"
	"github.com/otiai10/copy"
	"github.com/twitchtv/twirp"
	"golang.org/x/sync/errgroup"
)

// ReceiveCmd contains all the information necessary to execute receiving a job
// from the remote server.
type ReceiveCmd struct {
	ReceiveToken string
	BaseURL      string
}

// Execute runs the receive command, fetching the build context, download the
// source, compiling it into a slug, and finally uploading that final release
// artifact back to the slugcmplr server.
//
// TODO: Requests should set a sensible User-Agent.
// TODO: How to have client-side events/traces emitted securely? There is a HTTP exporter for Go.
// TODO: Clients should have sensible timeouts.
// TODO: Figure out pings and cancellation.
// We should ping the server to let it know we're alive every so often
// It can decide a reasonable interval to mark a job as dead and ensure
// killing it.
//
// The ping response can also be used to mark a job as cancelled and
// propagate that through the context.
// TODO: we should fetch a real cachedir and save it.
// TODO: Tarup cache and slug
// TODO: Upload Slug and Cache
// TODO: Mark Completion.
func (r *ReceiveCmd) Execute(ctx context.Context, out Outputter) error {
	c := jobs.NewJobsProtobufClient(r.BaseURL, http.DefaultClient)
	payload, err := c.Receive(ctx, &jobs.ReceiveRequest{ReceiveToken: r.ReceiveToken})
	if err != nil {
		return fmt.Errorf("error receive job: %w", err)
	}

	ctx, err = withAuth(ctx, payload.Token)
	if err != nil {
		return fmt.Errorf("error setting twirp headers: %w", err)
	}

	builddir, err := os.MkdirTemp("", "")
	if err != nil {
		return fmt.Errorf("failed to create tmpdir: %w", err)
	}

	sourcefile, sourcedir := filepath.Join(builddir, "source.tgz"), filepath.Join(builddir, "source")
	appdir := filepath.Join(builddir, "app")
	envdir := filepath.Join(builddir, "env")
	buildpacksdir := filepath.Join(builddir, "buildpacks")

	// PARALLEL
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		src, dst := payload.SourceUrl, sourcefile
		if err := download(gctx, src, dst); err != nil {
			return fmt.Errorf("error downloading source: %w", err)
		}

		if err := untargz(gctx, sourcefile, sourcedir); err != nil {
			return fmt.Errorf("error untargz source file: %w", err)
		}

		if err := copysrc(gctx, sourcedir, appdir); err != nil {
			return fmt.Errorf("error copying source: %w", err)
		}

		return nil
	})

	g.Go(func() error {
		// Dump env vars
		if err := writeenv(gctx, payload.ConfigVars, envdir); err != nil {
			return fmt.Errorf("error writing out env: %w", err)
		}

		return nil
	})

	var buildpacks []*buildpack.Buildpack
	g.Go(func() error {
		bps, err := downloadBuilpacks(gctx, payload.BuildpackUrn, buildpacksdir)
		if err != nil {
			return fmt.Errorf("error fetching buildpacks: %w", err)
		}

		buildpacks = bps

		return nil
	})
	if err := g.Wait(); err != nil {
		return err
	}

	cachedir, err := os.MkdirTemp("", "")
	if err != nil {
		return err
	}

	// Execute Build
	build := &buildpack.Build{
		CacheDir:      cachedir,
		BuildDir:      builddir,
		Stack:         payload.Stack,
		SourceVersion: payload.SourceVersion,
		Stdout:        out.OutOrStdout(),
		Stderr:        out.ErrOrStderr(),
	}

	detectedBuildpack := ""
	previousBuildpacks := make([]*buildpack.Buildpack, 0, len(buildpacks))

	for _, bp := range buildpacks {
		detected, ok, err := bp.Detect(ctx, build)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("buildpack detection failure: %v", bp.URL)
		}

		if err := bp.Compile(ctx, previousBuildpacks, build); err != nil {
			return err
		}

		previousBuildpacks = append(previousBuildpacks, bp)
		detectedBuildpack = detected
	}

	fmt.Println(detectedBuildpack)

	return nil
}

func download(ctx context.Context, url, dst string) error {
	f, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer f.Close() // nolint:errcheck

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
		return fmt.Errorf("failed while copying response to disk: %w", err)
	}

	if err := res.Body.Close(); err != nil {
		return fmt.Errorf("failed to close response body: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close destination file: %w", err)
	}

	return nil
}

func untargz(_ context.Context, source, dst string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer f.Close() // nolint:errcheck

	if err := buildpack.Untargz(f, dst, true); err != nil {
		return fmt.Errorf("failed to untargz source: %w", err)
	}

	return nil
}

func copysrc(_ context.Context, src, dst string) error {
	if err := os.MkdirAll(dst, 0700); err != nil {
		return fmt.Errorf("failed to mkdir (%v): %w", dst, err)
	}

	ignore, err := slugignore.ForDirectory(src)
	if err != nil {
		return fmt.Errorf("failed to read .slugignore: %w", err)
	}

	if err := copy.Copy(src, dst, copy.Options{
		Skip: func(path string) (bool, error) {
			return ignore.IsIgnored(
				strings.TrimPrefix(path, src),
			), nil
		},
	}); err != nil {
		return fmt.Errorf("failed to copy source: %w", err)
	}

	return nil
}

func writeenv(_ context.Context, env map[string]string, dst string) error {
	if err := os.MkdirAll(dst, 0700); err != nil {
		return fmt.Errorf("failed to mkdir (%v): %w", dst, err)
	}

	for name, value := range env {
		if err := os.WriteFile(filepath.Join(dst, name), []byte(value), 0600); err != nil {
			return fmt.Errorf("error writing %v: %w", name, err)
		}
	}

	return nil
}

func downloadBuilpacks(ctx context.Context, urns []string, dir string) ([]*buildpack.Buildpack, error) {
	g, gctx := errgroup.WithContext(ctx)

	bps := make([]*buildpack.Buildpack, len(urns))
	download := func(i int, urn string) func() error {
		return func() error {
			src, err := buildpack.ParseSource(urn)
			if err != nil {
				return fmt.Errorf("failed to parse buildpack urn (%v): %w", urn, err)
			}

			bp, err := src.Download(gctx, dir)
			if err != nil {
				return fmt.Errorf("failed to download buildpack (%v): %w", urn, err)
			}

			bps[i] = bp

			return nil
		}
	}

	for i, urn := range urns {
		g.Go(download(i, urn))
	}

	return bps, g.Wait()
}

func withAuth(ctx context.Context, token string) (context.Context, error) {
	h := make(http.Header)
	h.Set("Slugcmplr-Authorization", token)

	return twirp.WithHTTPRequestHeaders(ctx, h)
}
