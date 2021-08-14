package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

type Release struct {
	Application string `json:"application"`
	Slug        string `json:"slug"`
	Commit      string `json:"commit"`
}

func release(ctx context.Context, h *heroku.Service, buildDir string) error {
	step(os.Stdout, "Reading release")
	log(os.Stdout, "From: %v", filepath.Join(buildDir, "release.json"))

	rf, err := os.Open(filepath.Join(buildDir, "release.json"))
	if err != nil {
		return fmt.Errorf("failed to read release data: %w", err)
	}
	defer rf.Close()

	r := &Release{}
	if err := json.NewDecoder(rf).Decode(r); err != nil {
		return fmt.Errorf("failed to decode release data: %w", err)
	}

	log(os.Stdout, "application: %v", r.Application)
	log(os.Stdout, "slug: %v", r.Slug)

	step(os.Stdout, "Releasing slug %v to %v", r.Slug, r.Application)

	release, err := h.ReleaseCreate(ctx, r.Application, heroku.ReleaseCreateOpts{
		Slug:        r.Slug,
		Description: heroku.String(fmt.Sprintf("Deployed %v", r.Commit[:8])),
	})
	if err != nil {
		return fmt.Errorf("error creating release: %w", err)
	}

	if release.OutputStreamURL != nil {
		return outputStream(os.Stdout, *release.OutputStreamURL)
	} else {
		return fmt.Errorf("release outputStreamURL nil")
	}
}

func releaseCmd() *cobra.Command {
	var buildDir string
	cmd := &cobra.Command{
		Use:   "release",
		Short: "release a slug",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := netrcClient()
			if err != nil {
				return err
			}

			return release(cmd.Context(), client, buildDir)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	return cmd
}
