package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

type Release struct {
	Application string `json:"application"`
	Slug        string `json:"slug"`
	Commit      string `json:"commit"`
}

func release(ctx context.Context, cmd Outputter, h *heroku.Service, buildDir, application string) error {
	step(cmd, "Reading release")
	log(cmd, "From: %v", filepath.Join(buildDir, "release.json"))

	rf, err := os.Open(filepath.Join(buildDir, "release.json"))
	if err != nil {
		return fmt.Errorf("failed to read release data: %w", err)
	}
	defer rf.Close()

	r := &Release{}
	if err := json.NewDecoder(rf).Decode(r); err != nil {
		return fmt.Errorf("failed to decode release data: %w", err)
	}

	if application == "" {
		application = r.Application
	}

	log(cmd, "application: %v", application)
	log(cmd, "slug: %v", r.Slug)

	step(cmd, "Releasing slug %v to %v", r.Slug, r.Application)

	release, err := h.ReleaseCreate(ctx, application, heroku.ReleaseCreateOpts{
		Slug:        r.Slug,
		Description: heroku.String(fmt.Sprintf("Deployed %v", r.Commit[:8])),
	})
	if err != nil {
		return fmt.Errorf("error creating release: %w", err)
	}

	if release.OutputStreamURL != nil {
		if err := outputStream(cmd, os.Stdout, *release.OutputStreamURL); err != nil {
			return fmt.Errorf("failed to stream output: %w", err)
		}
	}

	for i := 0; i < 36; i++ {
		log(cmd, "checking release status... (attempt %v)", i+1)

		info, err := h.ReleaseInfo(ctx, r.Application, release.ID)
		if err != nil {
			return fmt.Errorf("failed to fetch release info: %w", err)
		}

		log(cmd, "status: %v", info.Status)

		switch info.Status {
		case "failed":
			return fmt.Errorf("release failed")
		case "succeeded":
			return nil
		case "pending":
			continue
		}

		time.Sleep(5 * time.Second)
	}

	return fmt.Errorf("release still pending after multiple attempts")
}

func releaseCmd() *cobra.Command {
	var buildDir, application string
	cmd := &cobra.Command{
		Use:   "release",
		Short: "release a slug",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := netrcClient(cmd)
			if err != nil {
				return err
			}

			return release(cmd.Context(), cmd, client, buildDir, application)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	cmd.Flags().StringVar(&application, "app", "", "Override the application to release to")

	return cmd
}
