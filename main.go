package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

var (
	verbose      bool
	compileAppID string
)

// rootCmd represents the base command when called without any subcommands

var rootCmd = &cobra.Command{
	Use:   "slugcmplr",
	Short: "slugcmplr helps you detach building and releasing Heroku applications",
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}

func section(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, "-----> %s\n", fmt.Sprintf(format, a...))
}

func log(w io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(w, "       %s\n", fmt.Sprintf(format, a...))
}

func debug(w io.Writer, format string, a ...interface{}) {
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
