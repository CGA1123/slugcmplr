package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

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

var rootCmd = &cobra.Command{
	Use:   "slugcmplr",
	Short: "slugcmplr helps you detach building and releasing Heroku applications",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
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
	r, err := git.PlainOpen(".")
	if err != nil {
		return "", err
	}

	hsh, err := r.ResolveRevision(plumbing.Revision("HEAD"))
	if err != nil {
		return "", err
	}

	return hsh.String(), nil
}

func outputStream(out io.Writer, stream string) error {
	resp, err := http.Get(stream)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("output stream returned HTTP status: %v", resp.Status)
	}

	scn := bufio.NewScanner(resp.Body)
	for scn.Scan() {
		fmt.Fprintf(out, "%v\n", scn.Text())
	}

	return scn.Err()
}

func netrcClient() (*heroku.Service, error) {
	rcfile, err := loadNetrc()
	if err != nil {
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
