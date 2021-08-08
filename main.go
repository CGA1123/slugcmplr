package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/bgentry/go-netrc/netrc"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

var (
	verbose      bool
	compileAppID string
)

func main() {
	cmd := Cmd()
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func step(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, "-----> %s\n", fmt.Sprintf(format, a...))
}

func log(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, "       %s\n", fmt.Sprintf(format, a...))
}

func wrn(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, " !!    %s\n", fmt.Sprintf(format, a...))
}

func dbg(w io.Writer, format string, a ...interface{}) {
	if verbose {
		log(w, format, a...)
	}
}

func commit() (string, error) {
	step(os.Stdout, "Fetching HEAD commit...")
	r, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		wrn(os.Stderr, "error detecting HEAD commit: %v", err)
		return "", err
	}

	hsh, err := r.ResolveRevision(plumbing.Revision("HEAD"))
	if err != nil {
		wrn(os.Stderr, "error detecting HEAD commit: %v", err)
		return "", err
	}

	return hsh.String(), nil
}

func outputStream(out io.Writer, stream string) error {
	return outputStreamAttempt(out, stream, 0)
}

func outputStreamAttempt(out io.Writer, stream string, attempt int) error {
	if attempt >= 5 {
		return fmt.Errorf("failed to fetch outputStream after 5 attempts")
	}

	resp, err := http.Get(stream)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		if resp.StatusCode == 404 {
			log(os.Stdout, "Output stream 404, likely the process is still starting up. Trying again in 2s...")
			time.Sleep(2 * time.Second)

			return outputStreamAttempt(out, stream, attempt+1)
		} else {
			return fmt.Errorf("output stream returned HTTP status: %v", resp.Status)
		}
	}

	scn := bufio.NewScanner(resp.Body)
	for scn.Scan() {
		fmt.Fprintf(out, "%v\n", scn.Text())
	}

	return scn.Err()
}

func netrcClient() (*heroku.Service, error) {
	step(os.Stdout, "Building client from .netrc...")
	rcfile, err := loadNetrc()
	if err != nil {
		wrn(os.Stderr, "error creating client from .netrc: %v", err)
		return nil, err
	}

	machine := rcfile.FindMachine("api.heroku.com")
	if machine == nil {
		return nil, fmt.Errorf("no .netrc entry for api.heroku.com found")
	}

	return heroku.NewService(&http.Client{
		Transport: &heroku.Transport{
			Username: machine.Login,
			Password: machine.Password}}), nil
}

func Cmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "slugcmplr",
		Short: "slugcmplr helps you detach building and releasing Heroku applications",
	}

	buildCmd := &cobra.Command{
		Use:   "build [application]",
		Short: "Triggers a build of your application.",
		Long: `The build command will create a clone of your target application and
create a standard Heroku build. The build will _not_ run the release task in
your Procfile if it is defined.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			production := args[0]
			compile := compileAppID

			client, err := netrcClient()
			if err != nil {
				return err
			}

			commit, err := commit()
			if err != nil {
				return err
			}

			return build(cmd.Context(), production, compile, commit, client)
		},
	}

	compileCmd := &cobra.Command{
		Use:   "compile [target]",
		Short: "compile the target applications",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			production := args[0]
			client, err := netrcClient()
			if err != nil {
				return err
			}

			commit, err := commit()
			if err != nil {
				return err
			}

			return compile(cmd.Context(), production, commit, client)
		},
	}

	releaseCmd := &cobra.Command{
		Use:   "release [target]",
		Short: "Promotes a release from your compiler app to your target app.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			production := args[0]
			compile := compileAppID

			client, err := netrcClient()
			if err != nil {
				return err
			}

			commit, err := commit()
			if err != nil {
				return err
			}

			return release(cmd.Context(), production, compile, commit, client)
		},
	}

	buildCmd.Flags().StringVar(&compileAppID, "compiler", "",
		"The Heroku application to compile on (required)")
	buildCmd.MarkFlagRequired("compiler")

	releaseCmd.Flags().StringVar(&compileAppID, "compiler", "", "The Heroku application compiled on (required)")
	releaseCmd.MarkFlagRequired("compiler")
	rootCmd.AddCommand(releaseCmd)

	rootCmd.AddCommand(compileCmd)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.AddCommand(buildCmd)

	return rootCmd
}

func loadNetrc() (*netrc.Netrc, error) {
	if fromEnv := os.Getenv("NETRC"); fromEnv != "" {
		return netrc.ParseFile(fromEnv)
	}

	u, err := user.Current()
	if err != nil {
		return nil, err
	}

	return netrc.ParseFile(filepath.Join(u.HomeDir, ".netrc"))
}

func ptrStr(ptr *string) string {
	if ptr == nil {
		return "<NIL>"
	}

	return *ptr
}
