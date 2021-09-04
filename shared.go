package slugcmplr

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

const (
	// StackReplacePattern is used to replace the stack name (e.g. heroku-20)
	// during slugcmplr work.
	StackReplacePattern = "%stack%"

	// StackNumberReplacePattern is used to replace the stack number (e.g. 20)
	// during slugcmplr work.
	StackNumberReplacePattern = "%stack-number%"
)

// BuildpackReference is a reference to a buildpack, containing its raw URL and
// Name.
type BuildpackReference struct {
	Name string
	URL  string
}

// Outputter mimics the interface implemented by *cobra.Command to inject
// custom Stdout and Stderr streams, while also allowing control over the
// verbosity of output.
type Outputter interface {
	OutOrStdout() io.Writer
	ErrOrStderr() io.Writer
	IsVerbose() bool
}

// OutputterFromCmd builds an Outputter based on a *cobra.Command.
func OutputterFromCmd(cmd *cobra.Command, verbose bool) Outputter {
	return &StdOutputter{
		Err:     cmd.ErrOrStderr(),
		Out:     cmd.OutOrStdout(),
		Verbose: verbose,
	}
}

// StdOutputter is an Outputter that will default to os.Stdout and os.Stderr.
type StdOutputter struct {
	Out     io.Writer
	Err     io.Writer
	Verbose bool
}

// IsVerbose returnes whether the StdOutputter is in Verbose mode.
func (o *StdOutputter) IsVerbose() bool {
	return o.Verbose
}

// OutOrStdout returns either the configured Out, or os.Stdout if it is nil.
func (o *StdOutputter) OutOrStdout() io.Writer {
	if o.Out == nil {
		return os.Stdout
	}

	return o.Out
}

// ErrOrStderr returns either the configured Err, or os.Stderr if it is nil.
func (o *StdOutputter) ErrOrStderr() io.Writer {
	if o.Err == nil {
		return os.Stdout
	}

	return o.Err
}

// StackImage builds an image name for the given stack.
//
// stack is expected to be in the form `heroku-N` where N is the stack number
// (e.g. 18, 20).
//
// img may container either `%stack%` or `%stack-number%` which will be
// replaced by StackImage with the full stack name or only the number
// accordingly.
func StackImage(img, stack string) string {
	stackNumber := strings.TrimPrefix(stack, "heroku-")

	return strings.ReplaceAll(
		strings.ReplaceAll(img, StackReplacePattern, stack),
		StackNumberReplacePattern,
		stackNumber,
	)
}

// Commit attempts to return the current resolved HEAD commit for the git
// repository at dir.
func Commit(dir string) (string, error) {
	r, err := git.PlainOpenWithOptions(dir, &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		return "", fmt.Errorf("error opening git directory: %w", err)
	}

	hsh, err := r.ResolveRevision(plumbing.Revision("HEAD"))
	if err != nil {
		return "", fmt.Errorf("error resolving HEAD revision: %w", err)
	}

	return hsh.String(), nil
}

// Tarball represents a GZipped Tar file.
type Tarball struct {
	Path     string
	Checksum string
}

// Targz will walk srcDirPath recursively and write the corresponding GZipped Tar
// Archive to the given writers.
func Targz(srcDirPath, dstDirPath string) (*Tarball, error) {
	f, err := os.Create(dstDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create tarfile: %w", err)
	}
	defer f.Close() // nolint:errcheck

	sha := sha256.New()
	mw := io.MultiWriter(sha, f)

	gzw := gzip.NewWriter(mw)
	defer gzw.Close() // nolint:errcheck

	tw := tar.NewWriter(gzw)
	defer tw.Close() // nolint:errcheck

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
		defer f.Close() // nolint:errcheck

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

	return &Tarball{
		Path:     dstDirPath,
		Checksum: fmt.Sprintf("SHA256:%v", hex.EncodeToString(sha.Sum(nil))),
	}, nil
}
