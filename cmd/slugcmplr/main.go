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
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cmd := Cmd()
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		fmt.Printf("error: %v\n", err)
		os.Exit(1)
	}
}

// Cmd configures the entrypoint to the slugcmplr CLI with all its subcommands.
func Cmd() *cobra.Command {
	var verbose bool

	rootCmd := &cobra.Command{
		Use:           "slugcmplr",
		Short:         "slugcmplr helps you detach building and releasing Heroku applications",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	cmds := []func(bool) *cobra.Command{
		prepareCmd,
		compileCmd,
		releaseCmd,
		imageCmd,
		versionCmd,
		serverCmd,
	}
	for _, cmd := range cmds {
		rootCmd.AddCommand(cmd(verbose))
	}

	return rootCmd
}

type outputter interface {
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
	IsVerbose() bool
}

func outputterFromCmd(cmd *cobra.Command, verbose bool) outputter {
	return &stdOutputter{
		Err:     cmd.ErrOrStderr(),
		Out:     cmd.OutOrStdout(),
		Verbose: verbose,
	}
}

func step(cmd outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), "-----> %s\n", fmt.Sprintf(format, a...))
}

func log(cmd outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), "       %s\n", fmt.Sprintf(format, a...))
}

func wrn(cmd outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.ErrOrStderr(), " !!    %s\n", fmt.Sprintf(format, a...))
}

func dbg(cmd outputter, format string, a ...interface{}) {
	if cmd.IsVerbose() {
		log(cmd, format, a...)
	}
}

func netrcClient(cmd outputter) (*heroku.Service, error) {
	step(cmd, "Building client from .netrc...")
	netrcpath, err := netrcPath()
	if err != nil {
		wrn(cmd, "error finding .netrc file path: %v", err)
		return nil, err
	}

	rcfile, err := netrc.ParseFile(netrcpath)
	if err != nil {
		wrn(cmd, "error creating client from .netrc: %v", err)
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

func netrcPath() (string, error) {
	if fromEnv := os.Getenv("NETRC"); fromEnv != "" {
		return fromEnv, nil
	}

	u, err := user.Current()
	if err != nil {
		return "", err
	}

	return filepath.Join(u.HomeDir, ".netrc"), nil
}

func outputStream(cmd outputter, out io.Writer, stream string) error {
	return outputStreamAttempt(cmd, out, stream, 0)
}

func outputStreamAttempt(cmd outputter, out io.Writer, stream string, attempt int) error {
	if attempt >= 10 {
		return fmt.Errorf("failed to fetch outputStream after 5 attempts")
	}

	resp, err := http.Get(stream) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint:errcheck

	if resp.StatusCode > 399 {
		if resp.StatusCode == 404 {
			log(cmd, "Output stream 404, likely the process is still starting up. Trying again in 5s...")
			time.Sleep(5 * time.Second)

			return outputStreamAttempt(cmd, out, stream, attempt+1)
		}

		return fmt.Errorf("output stream returned HTTP status: %v", resp.Status)
	}

	scn := bufio.NewScanner(resp.Body)
	for scn.Scan() {
		fmt.Fprintf(out, "%v\n", scn.Text())
	}

	return scn.Err()
}

type stdOutputter struct {
	Out     io.Writer
	Err     io.Writer
	Verbose bool
}

func (o *stdOutputter) IsVerbose() bool {
	return o.Verbose
}

func (o *stdOutputter) OutOrStdout() io.Writer {
	if o.Out == nil {
		return os.Stdout
	}

	return o.Out
}

func (o *stdOutputter) ErrOrStderr() io.Writer {
	if o.Err == nil {
		return os.Stdout
	}

	return o.Err
}

func requireEnv(names ...string) (map[string]string, error) {
	result := map[string]string{}
	missing := []string{}

	for _, name := range names {
		v, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
		} else {
			result[name] = v
		}
	}

	if len(missing) > 0 {
		return map[string]string{}, fmt.Errorf("variables not set: %v", missing)
	}

	return result, nil
}
