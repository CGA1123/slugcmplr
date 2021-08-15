package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	heroku "github.com/heroku/heroku-go/v5"
)

func Test_Prepare(t *testing.T) {
	t.Parallel()

	withHarness(t, "CGA1123/slugcmplr-fixture-binary", func(t *testing.T, appName, repoDir string, h *heroku.Service) {
		buildDir, err := os.MkdirTemp("", "CGA1123__slugmplr-fixture-binary_build_")
		if err != nil {
			t.Fatalf("failed to create build directory: %v", err)
		}
		defer os.RemoveAll(buildDir)

		prepareCmd := Cmd()
		prepareCmd.SetArgs([]string{"prepare", appName, "--build-dir", buildDir, "--verbose"})
		ok(t, prepareCmd.Execute())

		// expect meta.json to be created properly
		f, err := os.Open(filepath.Join(buildDir, "meta.json"))
		if err != nil {
			t.Fatalf("failed to open meta.json: %v", err)
		}

		meta := &Compile{}
		if err := json.NewDecoder(f).Decode(meta); err != nil {
			t.Fatalf("failed to decode meta.json: %v", err)
		}

		if meta.Application != appName {
			t.Fatalf("expected meta.Application to be %v got %v", appName, meta.Application)
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
				t.Logf("expect index %v to be %v go %v", i, expected[i], meta.Buildpacks[i].URL)
			}

			return eq
		}) {
			t.Fatalf("buildpacks were not equal!")
		}

		// TODO: expect env to have been dumped
		// TODO: expect source to have been copied (and slugcleanup to have run!)
		// TODO: expect buildpacks to have been downloaded
	})
}
