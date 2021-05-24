package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/bissyio/slugcmplr/cmplr"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build [application]",
	Short: "Triggers a build of your application.",
	Long: `The build command will create a clone of your target application and
create a standard Heroku build. The build will _not_ run the release task in
your Procfile if it is defined.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := cmplr.Client()
		if err != nil {
			return err
		}

		commit, err := commit()
		if err != nil {
			return err
		}

		section(os.Stdout, "Compiling %v via %v for commit %v", args[0], compileAppID, commit[:7])

		exist, err := procfileExist()
		if err != nil {
			return fmt.Errorf("error detecting procfile: %v", err)
		}

		if exist {
			log(os.Stdout, "Procfile detected. Escaping release task...")
			if err := cmplr.EscapeReleaseTask("./Procfile"); err != nil {
				return fmt.Errorf("error escaping release task: %v", err)
			}
		} else {
			log(os.Stdout, "No Procfile detected")
		}

		// Tar it up
		sha := sha256.New()
		archive := &bytes.Buffer{}

		section(os.Stdout, "Creating source code tarball...")
		if err := cmplr.Tar(".", tar.FormatGNU, archive, sha); err != nil {
			return fmt.Errorf("error creating tarball: %v", err)
		}

		checksum := "SHA256:" + hex.EncodeToString(sha.Sum(nil))

		log(os.Stdout, "Checksum: %v", checksum)
		log(os.Stdout, "Size: %v", archive.Len())

		section(os.Stdout, "Uploading source code tarball...")
		urls, err := cmplr.Upload(context.Background(), client, archive)
		if err != nil {
			return err
		}

		debug(os.Stdout, "Get URL: %v", urls.Get)
		debug(os.Stdout, "Put URL: %v", urls.Put)

		section(os.Stdout, "Synchronising %v to %v...", args[0], compileAppID)
		if err := cmplr.Synchronise(context.Background(), client, args[0], compileAppID); err != nil {
			return err
		}

		section(os.Stdout, "Creating compilation build...")
		build, err := client.BuildCreate(context.Background(), compileAppID, heroku.BuildCreateOpts{
			SourceBlob: struct {
				Checksum *string `json:"checksum,omitempty" url:"checksum,omitempty,key"`
				URL      *string `json:"url,omitempty" url:"url,omitempty,key"`
				Version  *string `json:"version,omitempty" url:"version,omitempty,key"`
			}{
				Checksum: heroku.String(checksum),
				URL:      heroku.String(urls.Get),
				Version:  heroku.String(commit)}})
		if err != nil {
			return err
		}

		return outputStream(os.Stdout, build.OutputStreamURL)
	},
}

func init() {
	buildCmd.Flags().StringVar(&compileAppID, "compiler", "", "The Heroku application to compile on (required)")
	buildCmd.MarkFlagRequired("compiler")

	rootCmd.AddCommand(buildCmd)
}

func procfileExist() (bool, error) {
	if fi, err := os.Stat("./Procfile"); err == nil {
		return fi.Mode().IsRegular(), nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, err
	}
}
