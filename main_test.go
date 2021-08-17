package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cga1123/slugcmplr/buildpack"
	heroku "github.com/heroku/heroku-go/v5"
)

func Test_Suite(t *testing.T) {
	t.Parallel()

	netrcF := setupNetrc(t)
	os.Setenv("NETRC", netrcF)
	defer os.Remove(netrcF)

	// nolint: paralleltest
	t.Run("End to end tests", func(t *testing.T) {
		t.Run("TestDetectFail", testDetectFail)
		t.Run("TestPrepare", testPrepare)
		t.Run("TestGo", testGo)
		t.Run("TestRails", testRails)
		t.Run("TestBinary", testBinary)
	})
}

func testPrepare(t *testing.T) {
	t.Parallel()

	withHarness(t, "CGA1123/slugcmplr-fixture-binary",
		func(t *testing.T, appName, repoDir string, h *heroku.Service) {
			buildDir, err := os.MkdirTemp("", "CGA1123__slugmplr-fixture-binary_build_")
			if err != nil {
				t.Fatalf("failed to create build directory: %v", err)
			}
			defer os.RemoveAll(buildDir)

			prepareCmd := Cmd()
			prepareCmd.SetArgs([]string{
				"prepare", appName,
				"--build-dir", buildDir})
			ok(t, prepareCmd.Execute())

			// expect meta.json to be created properly
			// TODO: Check SourceVersion?
			f, err := os.Open(filepath.Join(buildDir, "meta.json"))
			if err != nil {
				t.Fatalf("failed to open meta.json: %v", err)
			}

			meta := &Compile{}
			if err := json.NewDecoder(f).Decode(meta); err != nil {
				t.Fatalf("failed to decode meta.json: %v", err)
			}

			if meta.Application != appName {
				t.Fatalf("expected meta.Application to be %v got %v",
					appName, meta.Application)
			}

			if meta.Stack != "heroku-20" {
				t.Fatalf("expected meta.Stack to be heroku-20 got %v", meta.Stack)
			}

			expected := []string{
				"https://github.com/CGA1123/heroku-buildpack-bar",
				"https://github.com/CGA1123/heroku-buildpack-foo"}

			if !SliceEqual(expected, meta.Buildpacks, func(i int) bool {
				eq := expected[i] == meta.Buildpacks[i].URL
				if !eq {
					t.Logf("expect index %v to be %v go %v",
						i, expected[i], meta.Buildpacks[i].URL)
				}

				return eq
			}) {
				t.Fatalf("buildpacks were not equal!")
			}

			// make sure we've dumped config vars
			envDir := filepath.Join(buildDir, buildpack.EnvironmentDir)
			entries, err := os.ReadDir(envDir)
			if err != nil {
				t.Fatalf("failed to read env dir entries: %v", err)
			}

			expectedConfig := map[string]string{"PING": "PONG", "PONG": "PING"}
			actualConfig := map[string]string{}
			for _, entry := range entries {
				path := filepath.Join(envDir, entry.Name())
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("error reading %v", path)
				}

				actualConfig[entry.Name()] = string(b)
			}

			if len(expectedConfig) != len(actualConfig) {
				t.Fatalf("expected %v config vars, got %v",
					len(expectedConfig), len(actualConfig))
			}

			for k, v := range expectedConfig {
				av, ok := actualConfig[k]
				if !ok {
					t.Fatalf("expected key %v to be present, it was not.", k)
				}

				if v != av {
					t.Fatalf("expected key %v to be %v, got %v", k, v, av)
				}
			}
		})
}

func testDetectFail(t *testing.T) {
	// t.Parallel()

	// TODO: can't run this in parallel, because we're messing
	// with package level vars (os.Stdout) causing data-races across
	// goroutines!
	//
	// Really need to suck it up and inject an io.Writer for stderr/stdout
	// Can use these in combination?
	// - https://pkg.go.dev/github.com/spf13/cobra#Command.SetOut
	// - https://pkg.go.dev/github.com/spf13/cobra#Command.OutOrStdout
	// - https://pkg.go.dev/github.com/spf13/cobra#Command.OutOrStderr

	// TODO: we don't need a full harness here, we can skip the heroku parts and inject the required, stack, config vars, and buildpacks
	// maybe there's a mising withStubHarness function?
	withHarness(t, "CGA1123/slugcmplr-fixture-binary", func(t *testing.T, app, src string, h *heroku.Service) {
		pattn := strings.ReplaceAll("CGA1123/slugcmplr-fixture-binary", "/", "__") + "_"
		buildDir, err := os.MkdirTemp("", pattn)
		if err != nil {
			t.Fatalf("failed to create build director: %v", err)
		}
		defer os.RemoveAll(buildDir)

		if _, err := h.BuildpackInstallationUpdate(context.Background(), app, heroku.BuildpackInstallationUpdateOpts{
			Updates: []struct {
				Buildpack string `json:"buildpack" url:"buildpack,key"`
			}{
				{Buildpack: "https://github.com/CGA1123/heroku-buildpack-bar"},
				{Buildpack: "https://github.com/CGA1123/heroku-buildpack-detect-fail"},
				{Buildpack: "https://github.com/CGA1123/heroku-buildpack-foo"},
			},
		}); err != nil {
			t.Fatalf("failed to update buildpacks: %v", err)
		}

		// Prepare
		prepareCmd := Cmd()
		prepareCmd.SetArgs([]string{
			"prepare", app,
			"--build-dir", buildDir,
			"--source-dir", src})
		ok(t, prepareCmd.Execute())

		var compileErr error
		logs := withCapturedStdOut(t, func() {
			// Compile
			compileCmd := Cmd()
			compileCmd.SetArgs([]string{
				"compile",
				"--build-dir", buildDir,
				"--image", "ghcr.io/cga1123/slugcmplr:testing"})

			compileErr = compileCmd.Execute()
		})

		if strings.Contains(logs, "Found BAR exported") {
			t.Fatalf("expected logs not to contain evidence of running heroku-buildpack-foo")
		}

		if !strings.Contains(logs, "App not compatible with buildpack: https://github.com/CGA1123/heroku-buildpack-detect-fail") {
			t.Fatalf("expected logs to mention CGA1123/heroku-buildpack-detect-fail is not compatible")
		}

		if compileErr == nil {
			t.Fatalf("expected err to be non-nil")
		}
	})
}

func testBinary(t *testing.T) {
	t.Parallel()

	endToEndSmoke(t, "CGA1123/slugcmplr-fixture-binary")
}

func testGo(t *testing.T) {
	t.Parallel()

	endToEndSmoke(t, "CGA1123/slugcmplr-fixture-go")
}

func testRails(t *testing.T) {
	t.Parallel()

	endToEndSmoke(t, "CGA1123/slugcmplr-fixture-rails")
}

func endToEndSmoke(t *testing.T, fixture string) {
	t.Helper()

	withHarness(t, fixture, func(t *testing.T, app, src string, h *heroku.Service) {
		pattn := strings.ReplaceAll(fixture, "/", "__") + "_"
		buildDir, err := os.MkdirTemp("", pattn)
		if err != nil {
			t.Fatalf("failed to create build director: %v", err)
		}
		defer os.RemoveAll(buildDir)

		// Prepare
		prepareCmd := Cmd()
		prepareCmd.SetArgs([]string{
			"prepare", app,
			"--build-dir", buildDir,
			"--source-dir", src})
		ok(t, prepareCmd.Execute())

		// Compile
		compileCmd := Cmd()
		compileCmd.SetArgs([]string{
			"compile",
			"--build-dir", buildDir,
			"--image", "ghcr.io/cga1123/slugcmplr:testing"})
		ok(t, compileCmd.Execute())

		// Release
		releaseCmd := Cmd()
		releaseCmd.SetArgs([]string{
			"release",
			"--build-dir", buildDir})
		ok(t, releaseCmd.Execute())
	})
}
