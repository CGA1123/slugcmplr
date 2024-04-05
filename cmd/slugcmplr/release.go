package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cga1123/slugcmplr"
	"github.com/spf13/cobra"
)

type release struct {
	Application string `json:"application"`
	Slug        string `json:"slug"`
	Commit      string `json:"commit"`
}

func releaseCmd(verbose bool) *cobra.Command {
	var buildDir, application, commit string
	cmd := &cobra.Command{
		Use:   "release",
		Short: "release a slug",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			out := outputterFromCmd(cmd, verbose)
			h, err := netrcClient(out)
			if err != nil {
				return err
			}

			step(out, "Reading release")
			log(out, "From: %v", filepath.Join(buildDir, "release.json"))

			rf, err := os.Open(filepath.Join(buildDir, "release.json"))
			if err != nil {
				return fmt.Errorf("failed to read release data: %w", err)
			}
			defer rf.Close() // nolint:errcheck

			r := &release{}
			if err := json.NewDecoder(rf).Decode(r); err != nil {
				return fmt.Errorf("failed to decode release data: %w", err)
			}

			if application != "" {
				r.Application = application
			}

			if commit != "" {
				r.Commit = commit
			}

			log(out, "application: %v", r.Application)
			log(out, "slug: %v", r.Slug)

			step(out, "Releasing slug %v to %v", r.Slug, r.Application)

			releaseCmd := &slugcmplr.ReleaseCmd{
				Heroku:      h,
				Application: r.Application,
				SlugID:      r.Slug,
				Commit:      r.Commit,
			}

			release, err := releaseCmd.Execute(ctx, out)
			if err != nil {
				return fmt.Errorf("error creating release: %w", err)
			}

			if release.OutputStreamURL != nil {
				if err := outputStream(out, os.Stdout, *release.OutputStreamURL); err != nil {
					return fmt.Errorf("failed to stream output: %w", err)
				}
			}

			for i := 0; i < 36; i++ {
				log(out, "checking release status... (attempt %v)", i+1)

				info, err := h.ReleaseInfo(ctx, r.Application, release.ID)
				if err != nil {
					return fmt.Errorf("failed to fetch release info: %w", err)
				}

				log(out, "status: %v", info.Status)

				switch info.Status {
				case "failed":
					return fmt.Errorf("release failed")
				case "succeeded":
					return nil
				case "pending":
					time.Sleep(5 * time.Second)

					continue
				}
			}

			return fmt.Errorf("release still pending after multiple attempts")
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.MarkFlagRequired("build-dir") // nolint:errcheck

	cmd.Flags().StringVar(&commit, "commit", "", "Override the commit this release is associated with")
	cmd.Flags().StringVar(&application, "app", "", "Override the application to release to")

	return cmd
}
