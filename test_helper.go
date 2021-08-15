package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/bgentry/go-netrc/netrc"
	git "github.com/go-git/go-git/v5"
	heroku "github.com/heroku/heroku-go/v5"
)

func MapEqual(a, b map[string]interface{}, f func(string) bool) bool {
	for ka := range a {
		if !f(ka) {
			return false
		}
	}

	for kb := range b {
		if !f(kb) {
			return false
		}
	}

	return true
}

func SliceEqual(a, b interface{}, eq func(i int) bool) bool {
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

	if err := os.Chdir(dir); err != nil {
		return "", "", fmt.Errorf("failed to change directories: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}

	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return "", "", fmt.Errorf("failed create tmpfile")
	}
	defer tmp.Close()

	tarball, err := targz(cwd, tmp.Name())
	if err != nil {
		return "", "", fmt.Errorf("failed tarring directory: %v", err)
	}

	src, err := h.SourceCreate(context.Background())
	if err != nil {
		return "", "", fmt.Errorf("error creating source: %w", err)
	}

	if err := upload(context.Background(), http.MethodPut, src.SourceBlob.PutURL, tarball.path); err != nil {
		return "", "", fmt.Errorf("failed to upload test app: %v", err)
	}

	app, err := h.AppSetupCreate(context.Background(), heroku.AppSetupCreateOpts{
		SourceBlob: struct {
			Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
			URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
			Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
		}{
			URL:      heroku.String(src.SourceBlob.GetURL),
			Checksum: heroku.String(tarball.checksum),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to create application: %v", err)
	}

	t.Logf("created app for %v (%v)", fixture, app.App.Name)
	t.Logf("(%v) checking build status...", app.App.Name)

	info, err := waitForBuild(t, h, app)
	if info != nil && info.Build != nil {
		outputStream(os.Stdout, info.Build.OutputStreamURL)
	}

	if err != nil {
		return app.App.Name, dir, err
	}

	return app.App.Name, dir, nil
}

func waitForBuild(t *testing.T, h *heroku.Service, app *heroku.AppSetup) (*heroku.AppSetup, error) {
	id, name := app.ID, app.App.Name

	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Second)

		info, err := h.AppSetupInfo(context.Background(), id)
		if err != nil {
			return nil, fmt.Errorf("(%v) error fetching app info: %v", name, err)
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

	netrcF := setupNetrc(t)
	os.Setenv("NETRC", netrcF)
	defer os.Remove(netrcF)

	h, err := netrcClient()
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

func fetchBuildpacks(t *testing.T, h *heroku.Service, app string) []string {
	packs, err := h.BuildpackInstallationList(context.Background(), app, nil)
	ok(t, err)

	sort.Slice(packs, func(i, j int) bool {
		return packs[i].Ordinal < packs[j].Ordinal
	})

	names := make([]string, len(packs))
	for i, v := range packs {
		names[i] = v.Buildpack.Name
	}

	return names
}

func fetchFeats(t *testing.T, h *heroku.Service, app string) map[string]bool {
	feats, err := h.AppFeatureList(context.Background(), app, nil)
	ok(t, err)

	featMap := make(map[string]bool, len(feats))
	for _, feat := range feats {
		featMap[feat.Name] = feat.Enabled
	}

	return featMap
}

func latestReleaseLog(t *testing.T, h *heroku.Service, app string) string {
	releases, err := h.ReleaseList(context.Background(), app, &heroku.ListRange{
		Descending: true, Field: "version"})
	ok(t, err)

	t.Logf("found release(%v): %v", releases[0].Version, releases[0].Description)

	url := releases[0].OutputStreamURL
	if url == nil {
		t.Fatalf("latest release output stream url is nil.")
	}

	resp, err := http.Get(*url)
	ok(t, err)
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		t.Logf("failed to fetch release status: %v", resp.Status)
		t.Fail()

		return ""
	}

	b, err := io.ReadAll(resp.Body)
	ok(t, err)

	return string(b)
}
