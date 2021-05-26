package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	heroku "github.com/heroku/heroku-go/v5"
)

func release(production, compile, commit string, client *heroku.Service) error {
	step(os.Stdout, "Releasing %v from %v to %v", commit[:7], compile, production)
	log(os.Stdout, "Finding correct release...")
	releases, err := client.ReleaseList(context.Background(), compile, &heroku.ListRange{
		Descending: true, Field: "version"})
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

	log(os.Stdout, "Found release %v (v%v)", compileRelease.ID, compileRelease.Version)

	step(os.Stdout, "Releasing slug %v to %v", compileRelease.Slug.ID, production)
	prodRelease, err := client.ReleaseCreate(context.Background(), production, heroku.ReleaseCreateOpts{
		Slug: compileRelease.Slug.ID, Description: heroku.String(commit),
	})
	if err != nil {
		wrn(os.Stdout, "error promoting slug: %v", err)

		return err
	}

	if prodRelease.OutputStreamURL != nil {
		if err := outputStream(os.Stdout, *prodRelease.OutputStreamURL); err != nil {
			wrn(os.Stderr, "error streaming release logs: %v", err)
		}
	} else {
		dbg(os.Stdout, "No output stream for release %v", prodRelease.ID)
	}
	log(os.Stdout, "Done.")

	step(os.Stdout, "Verifying release status...")

	for i := 0; i < 5; i++ {
		release, err := client.ReleaseInfo(context.Background(), production, prodRelease.ID)
		if err != nil {
			wrn(os.Stderr, "error checking release state: %v", err)

			return err
		}

		switch status := release.Status; status {
		case "pending":
			log(os.Stderr, "release is still pending...")
			time.Sleep(5 * time.Second)
		case "failed":
			wrn(os.Stderr, "release failed, try again?")

			return fmt.Errorf("release failed, try again?")
		case "succeeded":
			log(os.Stdout, "release succeeded")

			return nil
		default:
			wrn(os.Stderr, "unknown release status: %v", status)

			return fmt.Errorf("unknown release status returned by Heroku: %v", status)
		}

	}

	wrn(os.Stderr, "release is still pending, aborting checks.")
	wrn(os.Stderr, "this may be due to Heroku being unable or slow to provision release dynos")
	wrn(os.Stderr, "or a very slow release task, check the Heroku logs or Heroku status pages")

	return fmt.Errorf("release is still pending after a while.")
}
