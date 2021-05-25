package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	heroku "github.com/heroku/heroku-go/v5"
)

func release(production, compile, commit string, client *heroku.Service) error {
	step(os.Stdout, "Releasing %v from %v to %v", commit[:7], compile, production)
	log(os.Stdout, "Finding correct release...")
	releases, err := client.ReleaseList(context.Background(), compile, nil)
	if err != nil {
		wrn(os.Stderr, "error fetching releases from %v: %v", compile, err)

		return err
	}
	description := fmt.Sprintf("Deploy %s", commit[:7])
	var compileRelease *heroku.Release
	for _, release := range releases {
		dbg(os.Stdout, "release ID: %v, release Desc: %v", release.ID, release.Description)

		if strings.HasPrefix(release.Description, description) {
			compileRelease = &release
			break
		}
	}
	if compileRelease == nil {
		wrn(os.Stderr, "failed to find a release for %v on %v", commit, compile)

		return fmt.Errorf("could not find release on compile app for %v", commit)
	}

	log(os.Stdout, "Found release %v", compileRelease.ID)

	step(os.Stdout, "Releasing slug %v to %v", compileRelease.Slug.ID, production)
	prodRelease, err := client.ReleaseCreate(context.Background(), production, heroku.ReleaseCreateOpts{
		Slug: compileRelease.Slug.ID, Description: heroku.String(commit),
	})
	if err != nil {
		wrn(os.Stdout, "error promoting slug: %v", err)

		return err
	}

	if prodRelease.OutputStreamURL != nil {
		return outputStream(os.Stdout, *prodRelease.OutputStreamURL)
	} else {
		dbg(os.Stdout, "No output stream for release %v", prodRelease.ID)
	}

	// TODO: Wait until release status is success.
	// If release fails, error.
	// Should there be a timeout on how long we wait for a release? (60s?)
	// Sometimes Heroku is having issues...

	return nil
}
