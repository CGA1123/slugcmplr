package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"sort"
	"testing"
	"time"

	"github.com/bgentry/go-netrc/netrc"
	heroku "github.com/heroku/heroku-go/v5"
)

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

func setupProdApp(t *testing.T, h *heroku.Service, fixture string) (string, error) {
	if err := os.Chdir("./fixtures/" + fixture); err != nil {
		return "", fmt.Errorf("failed to change directories: %v", err)
	}

	tarball, err := targz()
	if err != nil {
		return "", fmt.Errorf("failed tarring directory: %v", err)
	}

	src, err := upload(context.Background(), h, tarball.blob)
	if err != nil {
		return "", fmt.Errorf("failed to upload test app: %v", err)
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
		return "", fmt.Errorf("failed to create application: %v", err)
	}

	t.Logf("created app for %v (%v)", fixture, app.App.Name)
	t.Logf("(%v) checking build status...", app.App.Name)

	if err := waitForBuild(t, h, app); err != nil {
		return app.App.Name, err
	}

	return app.App.Name, nil
}

func waitForBuild(t *testing.T, h *heroku.Service, app *heroku.AppSetup) error {
	id, name := app.ID, app.App.Name

	for i := 0; i < 10; i++ {
		time.Sleep(10 * time.Second)

		info, err := h.AppSetupInfo(context.Background(), id)
		if err != nil {
			return fmt.Errorf("(%v) error fetching app info: %v", name, err)
		}

		t.Logf("(%v) status: %v", name, info.Status)

		if info.Status == "failed" {
			return fmt.Errorf("(%v) failed to setup test app: %v", name, info.FailureMessage)
		}

		if info.Status == "succeeded" {
			return nil
		}
	}

	return fmt.Errorf("(%v) build still pending after a long time, aborting", name)
}

func destroyApp(t *testing.T, h *heroku.Service, app string) {
	t.Logf("destroying %v...", app)

	_, err := h.AppDelete(context.Background(), app)
	if err != nil {
		t.Logf("failed to destroy %v: %v", app, err)
	}

	t.Logf("destroyed %v", app)
}

func setupCompileApp(t *testing.T, h *heroku.Service) string {
	app, err := h.AppCreate(context.Background(), heroku.AppCreateOpts{})
	if err != nil {
		t.Fatalf("error creating compile app: %v", err)
	}

	t.Logf("created compile app %v", app.Name)

	return app.Name
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

	production, err := setupProdApp(t, h, fixture)
	if err != nil {
		if production != "" {
			destroyApp(t, h, production)
		}

		t.Fatalf("failed to setup production application: %v", err)
	}

	defer destroyApp(t, h, production)

	compile := setupCompileApp(t, h)
	defer destroyApp(t, h, compile)

	f(t, production, compile, h)
}

func Test_Build(t *testing.T) {
	t.Parallel()

	withHarness(t, "go-simple", func(t *testing.T, production, compile string, h *heroku.Service) {
		cmd := Cmd()
		cmd.SetArgs([]string{"build", production, "--compiler", compile, "--verbose"})

		ok(t, cmd.ExecuteContext(context.Background()))

		prodConf, err := h.ConfigVarInfoForApp(context.Background(), production)
		ok(t, err)

		compileConf, err := h.ConfigVarInfoForApp(context.Background(), compile)
		ok(t, err)

		t.Log("checking env vars...")
		// Expect all prod env vars to be synchronised to the compile app
		for k, pv := range prodConf {
			cv, ok := compileConf[k]
			if !ok {
				t.Logf("%v missing on compile app", k)
				t.Fail()

				continue
			}

			if *cv != *pv {
				t.Logf("expected %v for key %v, got %v", pv, k, cv)
				t.Fail()

				continue
			}
		}

		// Expect only 1 addtional env var on compile app
		if (len(compileConf) - len(prodConf)) != 1 {
			t.Logf("expected compile app to have only 1 additional environment variable (SLUGCMPLR)")
			t.Logf("compile: %v", compileConf)
			t.Logf("production: %v", prodConf)

			t.Fail()
		}

		// Expect SLUGCMPLR to be set on compile app
		if _, ok := compileConf["SLUGCMPLR"]; !ok {
			t.Logf("expected compile app to have SLUGCMPLR set")

			t.Fail()
		}

		t.Log("checking buildpacks...")
		// expect buildpacks on compile app to be set
		prodBuildpack := fetchBuildpacks(t, h, production)
		compileBuildpack := fetchBuildpacks(t, h, compile)

		if !SliceEqual(prodBuildpack, compileBuildpack, func(i int) bool {
			return prodBuildpack[i] == compileBuildpack[i]
		}) {
			t.Logf("expected buildpack on both apps to match")
			t.Logf("production: %v", prodBuildpack)
			t.Logf("compile: %v", compileBuildpack)

			t.Fail()
		}

		t.Log("checking release log...")
		// expect release command not to have run
		log := latestReleaseLog(t, h, compile)

		matched, err := regexp.MatchString("RELEASE COMMAND RAN", log)
		ok(t, err)

		if matched {
			t.Logf("expected release to have been escaped")
			t.Logf("===== LOG BEGIN =====")
			t.Logf(log)
			t.Logf("===== LOG END =====")

			t.Fail()
		}
	})
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
