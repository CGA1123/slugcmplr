package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cga1123/slugcmplr"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
)

func writeMetadata(m *slugcmplr.MetadataResult, pr *slugcmplr.PrepareResult) error {
	metafile := filepath.Join(pr.BuildDir, "meta.json")

	c := &Compile{
		Application:   m.ApplicationName,
		Stack:         m.Stack,
		SourceVersion: m.SourceVersion,
		Buildpacks:    pr.Buildpacks,
	}

	b := &bytes.Buffer{}
	if err := json.NewEncoder(b).Encode(c); err != nil {
		return fmt.Errorf("error encoding metadata: %w", err)
	}

	if err := os.WriteFile(metafile, b.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed to create meta file: %w", err)
	}

	return nil
}

func prepareCmd(verbose bool) *cobra.Command {
	var buildDir, srcDir string

	cmd := &cobra.Command{
		Use:   "prepare [target]",
		Short: "prepare the target application for compilation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			application := args[0]
			output := outputterFromCmd(cmd, verbose)
			h, err := netrcClient(output)
			if err != nil {
				return err
			}

			if buildDir == "" {
				bd, err := os.MkdirTemp("", "")
				if err != nil {
					return err
				}

				buildDir = bd
			}

			if srcDir == "" {
				sd, err := os.Getwd()
				if err != nil {
					return err
				}

				srcDir = sd
			}

			step(output, "Preparing app: %v", application)

			m, err := (&slugcmplr.MetadataCmd{
				Heroku:      h,
				Application: application,
				SourceDir:   srcDir,
				BuildDir:    buildDir,
				Tracer:      otel.Tracer("github.com/CGA1123/slugcmplr/cmd"),
			}).Execute(ctx, output)
			if err != nil {
				return fmt.Errorf("error fetching app metadata: %w", err)
			}

			log(output, "stack: %v", m.Stack)
			log(output, "%v config vars", len(m.ConfigVars))
			log(output, "%v buildpacks", len(m.Buildpacks))
			log(output, "commit: %v", commit)

			pr, err := (&slugcmplr.PrepareCmd{
				SourceDir:  srcDir,
				BuildDir:   buildDir,
				ConfigVars: m.ConfigVars,
				Buildpacks: m.Buildpacks,
				Tracer:     otel.Tracer("github.com/CGA1123/slugcmplr/cmd"),
			}).Execute(ctx, output)
			if err != nil {
				return fmt.Errorf("error preparing application: %w", err)
			}

			step(output, "Writing metadata")

			return writeMetadata(m, pr)
		},
	}

	cmd.Flags().StringVar(&buildDir, "build-dir", "", "The build directory")
	cmd.Flags().StringVar(&srcDir, "source-dir", "", "The source app directory")

	return cmd
}
