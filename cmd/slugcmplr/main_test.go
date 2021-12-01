package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cga1123/slugcmplr"
	"github.com/cga1123/slugcmplr/buildpack"
	heroku "github.com/heroku/heroku-go/v5"
)

func Test_Suite(t *testing.T) {
	t.Parallel()

	netrcF := setupNetrc(t)
	if err := os.Setenv("NETRC", netrcF); err != nil {
		t.Fatalf("error setting NETRC: %v", err)
	}
	defer os.Remove(netrcF) // nolint:errcheck

	// nolint: paralleltest
	t.Run("End to end tests", func(t *testing.T) {
		t.Run("TestDetectFail", testDetectFail)
		t.Run("TestSlugIgnore", testSlugIgnore)
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
			defer os.RemoveAll(buildDir) // nolint:errcheck

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

			if !sliceEqual(expected, meta.Buildpacks, func(i int) bool {
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
	t.Parallel()

	buildpacks := []*slugcmplr.BuildpackReference{
		{URL: "https://github.com/CGA1123/heroku-buildpack-bar", Name: "CGA1123/heroku-buildpack-bar"},
		{URL: "https://github.com/CGA1123/heroku-buildpack-detect-fail", Name: "CGA1123/heroku-buildpack-detect-fail"},
		{URL: "https://github.com/CGA1123/heroku-buildpack-foo", Name: "CGA1123/heroku-buildpack-foo"},
	}

	configVars := map[string]string{"FOO": "BAR", "BAR": "FOO"}

	withStubPrepare(t, "CGA1123/slugcmplr-fixture-binary", buildpacks, configVars, func(t *testing.T, app, buildDir string) {
		var compileErr error
		// Compile
		logBuilder := &strings.Builder{}
		writer := io.MultiWriter(logBuilder, os.Stdout)

		compileCmd := Cmd()
		compileCmd.SetOut(writer)
		compileCmd.SetErr(writer)
		compileCmd.SetArgs([]string{
			"compile",
			"--build-dir", buildDir})

		compileErr = compileCmd.Execute()

		logs := logBuilder.String()

		if strings.Contains(logs, "Found BAR exported") {
			t.Fatalf("expected logs not to contain evidence of running heroku-buildpack-foo")
		}

		if !strings.Contains(logs, "buildpack detection failure: https://github.com/CGA1123/heroku-buildpack-detect-fail") {
			t.Fatalf("expected logs to mention CGA1123/heroku-buildpack-detect-fail is not compatible")
		}

		if compileErr == nil {
			t.Fatalf("expected err to be non-nil")
		}
	})
}

func testSlugIgnore(t *testing.T) {
	t.Parallel()

	buildpacks := []*slugcmplr.BuildpackReference{
		{URL: "https://github.com/CGA1123/heroku-buildpack-bar", Name: "CGA1123/heroku-buildpack-bar"},
		{URL: "https://github.com/CGA1123/heroku-buildpack-foo", Name: "CGA1123/heroku-buildpack-foo"},
	}

	configVars := map[string]string{"FOO": "BAR", "BAR": "FOO"}

	withStubPrepare(t, "CGA1123/slugcmplr-fixture-slugignore", buildpacks, configVars, func(t *testing.T, app, buildDir string) {
		foundPaths := []string{}
		expectedPaths := []string{
			"/README.md",
			"/.slugignore",
			"/keep-me/hello.txt",
			"/vendor/keep-this-dir/file-1.txt",
			"/vendor/keep-this-dir/file-2.txt",
		}

		err := filepath.Walk(filepath.Join(buildDir, buildpack.AppDir), func(path string, info os.FileInfo, err error) error {
			if err != nil {
				t.Fatalf("error while walking directory: %v", err)
			}

			if info.Mode().IsRegular() {
				foundPaths = append(foundPaths, strings.TrimPrefix(path, filepath.Join(buildDir, buildpack.AppDir)))
			}

			return nil
		})
		if err != nil {
			t.Fatalf("filepath.Walk error: %v", err)
		}

		sort.Strings(foundPaths)
		sort.Strings(expectedPaths)

		if !sliceEqual(foundPaths, expectedPaths, func(i int) bool {
			return foundPaths[i] == expectedPaths[i]
		}) {
			expected := strings.Join(expectedPaths, "\n")
			actual := strings.Join(foundPaths, "\n")

			t.Fatalf("\nexpected:\n%v\n---\nactual:\n%v\n", expected, actual)
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
		defer os.RemoveAll(buildDir) // nolint:errcheck

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
			"--build-dir", buildDir})
		ok(t, compileCmd.Execute())

		// Release
		releaseCmd := Cmd()
		releaseCmd.SetArgs([]string{
			"release",
			"--build-dir", buildDir})
		ok(t, releaseCmd.Execute())
	})
}
