package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/bgentry/go-netrc/netrc"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func Cmd() *cobra.Command {
	var verbose bool

	rootCmd := &cobra.Command{
		Use:           "slugcmplr",
		Short:         "slugcmplr helps you detach building and releasing Heroku applications",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	rootCmd.AddCommand(prepareCmd(verbose))
	rootCmd.AddCommand(compileCmd(verbose))
	rootCmd.AddCommand(releaseCmd(verbose))
	rootCmd.AddCommand(imageCmd(verbose))
	rootCmd.AddCommand(versionCmd(verbose))

	return rootCmd
}

type Outputter interface {
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
	IsVerbose() bool
}

func OutputterFromCmd(cmd *cobra.Command, verbose bool) Outputter {
	return &outputter{
		Err:     cmd.ErrOrStderr(),
		Out:     cmd.OutOrStdout(),
		Verbose: verbose,
	}
}

func step(cmd Outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), "-----> %s\n", fmt.Sprintf(format, a...))
}

func log(cmd Outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.OutOrStdout(), "       %s\n", fmt.Sprintf(format, a...))
}

func wrn(cmd Outputter, format string, a ...interface{}) {
	fmt.Fprintf(cmd.ErrOrStderr(), " !!    %s\n", fmt.Sprintf(format, a...))
}

func dbg(cmd Outputter, format string, a ...interface{}) {
	if cmd.IsVerbose() {
		log(cmd, format, a...)
	}
}

func commitDir(cmd Outputter, dir string) (string, error) {
	step(cmd, "Fetching HEAD commit...")
	r, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		wrn(cmd, "error detecting HEAD commit: %v", err)
		return "", err
	}

	hsh, err := r.ResolveRevision(plumbing.Revision("HEAD"))
	if err != nil {
		wrn(cmd, "error detecting HEAD commit: %v", err)
		return "", err
	}

	return hsh.String(), nil
}

func netrcClient(cmd Outputter) (*heroku.Service, error) {
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

type tarball struct {
	path     string
	checksum string
}

// targz will walk srcDirPath recursively and write the correspoding G-Zipped Tar
// Archive to the given writers.
func targz(srcDirPath, dstDirPath string) (*tarball, error) {
	f, err := os.Create(dstDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create tarfile: %w", err)
	}
	defer f.Close()

	sha := sha256.New()
	mw := io.MultiWriter(sha, f)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	walk := func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("file moved or removed while building tarball: %w", err)
		}

		link := ""
		isSymlink := false

		if (info.Mode() & fs.ModeSymlink) != 0 {
			l, err := os.Readlink(file)
			if err != nil {
				return fmt.Errorf("failed to readlink: %w", err)
			}

			link = l
			isSymlink = true
		}

		if !(info.Mode().IsRegular() || isSymlink) {
			return nil
		}

		header, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}

		// Heroku requires GNU Tar format (at least for slugs, maybe not for build sources?)
		//
		// https://devcenter.heroku.com/articles/platform-api-deploying-slugs#create-slug-archive
		header.Format = tar.FormatGNU
		header.Name = strings.TrimPrefix(strings.TrimPrefix(file, srcDirPath), string(filepath.Separator))

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if isSymlink {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		return f.Close()
	}

	if err := filepath.WalkDir(srcDirPath, walk); err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	// explicitly close to ensure we flush to archive and sha, make sure we get
	// a correct checksum.
	if err := tw.Close(); err != nil {
		return nil, err
	}

	if err := gzw.Close(); err != nil {
		return nil, err
	}

	if err := f.Close(); err != nil {
		return nil, err
	}

	return &tarball{
		path:     dstDirPath,
		checksum: fmt.Sprintf("SHA256:%v", hex.EncodeToString(sha.Sum(nil))),
	}, nil
}

func upload(ctx context.Context, method, url, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, f)
	if err != nil {
		return err
	}

	req.ContentLength = fi.Size()

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	var body string
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	body = string(b)

	if response.StatusCode > 399 {
		return fmt.Errorf("HTTP %v: %v", response.Status, body)
	}

	return nil
}

func outputStream(cmd Outputter, out io.Writer, stream string) error {
	return outputStreamAttempt(cmd, out, stream, 0)
}

func outputStreamAttempt(cmd Outputter, out io.Writer, stream string, attempt int) error {
	if attempt >= 10 {
		return fmt.Errorf("failed to fetch outputStream after 5 attempts")
	}

	resp, err := http.Get(stream) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close()

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

type outputter struct {
	Out     io.Writer
	Err     io.Writer
	Verbose bool
}

func (o *outputter) IsVerbose() bool {
	return o.Verbose
}

func (o *outputter) OutOrStdout() io.Writer {
	if o.Out == nil {
		return os.Stdout
	}

	return o.Out
}

func (o *outputter) ErrOrStderr() io.Writer {
	if o.Err == nil {
		return os.Stdout
	}

	return o.Err
}
