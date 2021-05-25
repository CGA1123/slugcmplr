package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

var releaseCmd = &cobra.Command{
	Use:   "release [target]",
	Short: "Promotes a release from your compiler app to your target app.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := netrcClient()
		if err != nil {
			return err
		}

		hash, err := commit()
		if err != nil {
			return err
		}

		step(os.Stdout, "Releasing %v from %v to %v", hash[:7], compileAppID, args[0])
		log(os.Stdout, "Finding correct release...")
		releases, err := client.ReleaseList(context.Background(), compileAppID, nil)
		if err != nil {
			return err
		}
		description := fmt.Sprintf("Deploy %s", hash[:7])
		var compileRelease *heroku.Release
		for _, release := range releases {
			if strings.HasPrefix(release.Description, description) {
				compileRelease = &release
				break
			}
		}
		if compileRelease == nil {
			return fmt.Errorf("could not find release on compile app for %v", hash)
		}

		log(os.Stdout, "Found release %v", compileRelease.ID)

		step(os.Stdout, "Releasing slug %v to %v", compileRelease.Slug.ID, args[0])
		prodRelease, err := client.ReleaseCreate(context.Background(), args[0], heroku.ReleaseCreateOpts{
			Slug: compileRelease.Slug.ID, Description: heroku.String(hash),
		})
		if err != nil {
			return err
		}

		if prodRelease.OutputStreamURL != nil {
			return outputStream(os.Stdout, *prodRelease.OutputStreamURL)
		}

		return nil
	},
}

func init() {
	releaseCmd.Flags().StringVar(&compileAppID, "compiler", "", "The Heroku application compiled on (required)")
	releaseCmd.MarkFlagRequired("compiler")
	rootCmd.AddCommand(releaseCmd)
}
