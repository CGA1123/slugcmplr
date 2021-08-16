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
	verbose bool
)

func main() {
	cmd := Cmd()
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		os.Exit(1)
	}
}

func Cmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "slugcmplr",
		Short: "slugcmplr helps you detach building and releasing Heroku applications",
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")

	rootCmd.AddCommand(prepareCmd())
	rootCmd.AddCommand(compileCmd())
	rootCmd.AddCommand(releaseCmd())

	return rootCmd
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

func commitDir(dir string) (string, error) {
	step(os.Stdout, "Fetching HEAD commit...")
	r, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
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

func netrcClient() (*heroku.Service, error) {
	step(os.Stdout, "Building client from .netrc...")
	netrcpath, err := netrcPath()
	if err != nil {
		wrn(os.Stderr, "error finding .netrc file path: %v", err)
		return nil, err
	}

	rcfile, err := netrc.ParseFile(netrcpath)
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
//
// TODO: symlinks?
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

		if !info.Mode().IsRegular() {
			return nil
		}

		header, err := tar.FileInfoHeader(info, d.Name())
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(strings.TrimPrefix(file, srcDirPath), string(filepath.Separator))

		// Heroku requires GNU Tar format (at least for slugs, maybe not for build sources?)
		//
		// https://devcenter.heroku.com/articles/platform-api-deploying-slugs#create-slug-archive
		header.Format = tar.FormatGNU

		if err := tw.WriteHeader(header); err != nil {
			return err
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

func outputStream(out io.Writer, stream string) error {
	return outputStreamAttempt(out, stream, 0)
}

func outputStreamAttempt(out io.Writer, stream string, attempt int) error {
	if attempt >= 5 {
		return fmt.Errorf("failed to fetch outputStream after 5 attempts")
	}

	resp, err := http.Get(stream) // #nosec G107
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 399 {
		if resp.StatusCode == 404 {
			log(os.Stdout, "Output stream 404, likely the process is still starting up. Trying again in 2s...")
			time.Sleep(2 * time.Second)

			return outputStreamAttempt(out, stream, attempt+1)
		}

		return fmt.Errorf("output stream returned HTTP status: %v", resp.Status)
	}

	scn := bufio.NewScanner(resp.Body)
	for scn.Scan() {
		fmt.Fprintf(out, "%v\n", scn.Text())
	}

	return scn.Err()
}
