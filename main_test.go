package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cga1123/slugcmplr/buildpack"
	heroku "github.com/heroku/heroku-go/v5"
)

func Test_Prepare(t *testing.T) {
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
				"--build-dir", buildDir,
				"--verbose"})
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

			// Compile
			compileCmd := Cmd()
			compileCmd.SetArgs([]string{
				"compile",
				"--build-dir", buildDir,
				"--verbose"})
			ok(t, compileCmd.Execute())
		})
}
