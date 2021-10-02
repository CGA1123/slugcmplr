package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bgentry/go-netrc/netrc"
	"github.com/cga1123/slugcmplr"
	git "github.com/go-git/go-git/v5"
	heroku "github.com/heroku/heroku-go/v5"
	"go.opentelemetry.io/otel"
)

func sliceEqual(a, b interface{}, eq func(i int) bool) bool {
	av, bv := reflect.ValueOf(a), reflect.ValueOf(b)
	if av.Len() != bv.Len() {
		return false
	}

	for i := 0; i < av.Len(); i++ {
		if !eq(i) {
			return false
		}
	}

	return true
}

func acceptance(t *testing.T) {
	if _, ok := os.LookupEnv("SLUGCMPLR_ACC"); ok {
		return
	}

	t.Skipf("set SLUGCMPLR_ACC to run acceptance tests")
}

func setupNetrc(t *testing.T) string {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatalf("failed to create tmp file for netrc: %v", err)
	}

	net := netrc.Netrc{}
	net.NewMachine(
		"api.heroku.com",
		os.Getenv("SLUGCMPLR_ACC_HEROKU_EMAIL"),
		os.Getenv("SLUGCMPLR_ACC_HEROKU_PASS"),
		"")
	data, err := net.MarshalText()
	if err != nil {
		t.Fatalf("failed to marshal netrc: %v", err)
	}

	if _, err := tmp.Write(data); err != nil {
		t.Fatalf("failed writing netrc: %v", err)
	}

	if err := tmp.Close(); err != nil {
		t.Fatalf("failed closing netrc: %v", err)
	}

	return tmp.Name()
}

func setupApp(t *testing.T, h *heroku.Service, fixture string) (string, string, error) {
	dir, err := os.MkdirTemp("", strings.ReplaceAll(fixture, "/", "__")+"_")
	if err != nil {
		return "", "", fmt.Errorf("failed to create tempdir: %w", err)
	}

	t.Logf("tempdir for %v created: %v", fixture, dir)

	if _, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:   "https://github.com/" + fixture,
		Depth: 1,
	}); err != nil {
		return "", "", fmt.Errorf("failed to clone %v: %w", fixture, err)
	}

	t.Logf("cloned %v into: %v", fixture, dir)

	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return "", "", fmt.Errorf("failed create tmpfile")
	}
	defer tmp.Close() // nolint:errcheck

	tarball, err := slugcmplr.Targz(dir, tmp.Name())
	if err != nil {
		return "", "", fmt.Errorf("failed tarring directory: %w", err)
	}

	src, err := h.SourceCreate(context.Background())
	if err != nil {
		return "", "", fmt.Errorf("error creating source: %w", err)
	}

	if err := slugcmplr.UploadBlob(
		context.Background(),
		http.MethodPut,
		src.SourceBlob.PutURL,
		tarball.Path,
	); err != nil {
		return "", "", fmt.Errorf("failed to upload test app: %w", err)
	}

	app, err := h.AppSetupCreate(context.Background(), heroku.AppSetupCreateOpts{
		SourceBlob: struct {
			Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
			URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
			Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
		}{
			URL:      heroku.String(src.SourceBlob.GetURL),
			Checksum: heroku.String(tarball.Checksum),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create application: %w", err)
	}

	t.Logf("created app for %v (%v)", fixture, app.App.Name)
	t.Logf("(%v) checking build status...", app.App.Name)

	info, err := waitForBuild(t, h, app)
	if info != nil && info.Build != nil {
		if err := outputStream(&stdOutputter{}, os.Stdout, info.Build.OutputStreamURL); err != nil {
			return app.App.Name, dir, fmt.Errorf("failed to output build log: %w", err)
		}
	}

	if err != nil {
		return app.App.Name, dir, err
	}

	return app.App.Name, dir, nil
}

func waitForBuild(t *testing.T, h *heroku.Service, app *heroku.AppSetup) (*heroku.AppSetup, error) {
	id, name := app.ID, app.App.Name

	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Second)

		info, err := h.AppSetupInfo(context.Background(), id)
		if err != nil {
			return nil, fmt.Errorf("(%v) error fetching app info: %w", name, err)
		}

		t.Logf("(%v) status: %v", name, info.Status)

		if info.Status == "failed" {
			return info, fmt.Errorf("(%v) failed to setup test app: %v", name, info.FailureMessage)
		}

		if info.Status == "succeeded" {
			return info, nil
		}
	}

	return nil, fmt.Errorf("(%v) build still pending after a long time, aborting", name)
}

func destroyApp(t *testing.T, h *heroku.Service, app string) {
	t.Logf("destroying %v...", app)

	_, err := h.AppDelete(context.Background(), app)
	if err != nil {
		t.Logf("failed to destroy %v: %v", app, err)
	}

	t.Logf("destroyed %v", app)
}

func ok(t *testing.T, err error) {
	if err == nil {
		return
	}

	t.Fatalf(err.Error())
}

func withHarness(t *testing.T, fixture string, f func(*testing.T, string, string, *heroku.Service)) {
	acceptance(t)

	h, err := netrcClient(&stdOutputter{})
	ok(t, err)

	production, dir, err := setupApp(t, h, fixture)
	if err != nil {
		if production != "" {
			destroyApp(t, h, production)
		}

		t.Fatalf("failed to setup production application: %v", err)
	}

	defer destroyApp(t, h, production)

	f(t, production, dir, h)
}

func withStubPrepare(t *testing.T, fixture string, buildpacks []*slugcmplr.BuildpackReference, configVars map[string]string, f func(*testing.T, string, string)) {
	acceptance(t)

	srcdir, err := os.MkdirTemp("", strings.ReplaceAll(fixture, "/", "__")+"_")
	if err != nil {
		t.Fatalf("failed to create tempdir: %v", err)
	}
	defer os.RemoveAll(srcdir) // nolint:errcheck

	t.Logf("tempdir for %v created: %v", fixture, srcdir)

	if _, err := git.PlainClone(srcdir, false, &git.CloneOptions{
		URL:   "https://github.com/" + fixture,
		Depth: 1,
	}); err != nil {
		t.Fatalf("failed to clone %v: %v", fixture, err)
	}

	builddir, err := os.MkdirTemp("", strings.ReplaceAll(fixture, "/", "__")+"_build_")
	if err != nil {
		t.Fatalf("failed to create tempdir: %v", err)
	}
	defer os.RemoveAll(builddir) // nolint:errcheck

	commit, err := slugcmplr.Commit(srcdir)
	if err != nil {
		t.Fatalf("failed to fetch HEAD: %v", err)
	}

	m := &slugcmplr.MetadataResult{
		ApplicationName: fixture,
		Stack:           "heroku-20",
		Buildpacks:      buildpacks,
		ConfigVars:      configVars,
		SourceVersion:   commit,
	}

	p := &slugcmplr.PrepareCmd{
		SourceDir:  srcdir,
		BuildDir:   builddir,
		ConfigVars: m.ConfigVars,
		Buildpacks: m.Buildpacks,
		Tracer:     otel.Tracer("github.com/slugcmlr/test"),
	}

	pr, err := p.Execute(context.Background(), &stdOutputter{})
	if err != nil {
		t.Fatalf("error preparing application: %v", err)
	}

	if err := writeMetadata(m, pr); err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	f(t, fixture, builddir)
}
