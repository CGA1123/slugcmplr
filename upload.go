package slugcmplr

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	heroku "github.com/heroku/heroku-go/v5"
)

// UploadCmd wraps up all the information required to upload a slug to a
// particular Heroku application.
type UploadCmd struct {
	Heroku            *heroku.Service
	Application       string
	Checksum          string
	Path              string
	DetectedBuildpack string
	SourceVersion     string
	Stack             string
	ProcessTypes      map[string]string
}

// UploadResult returns metadata about the uploaded slug, so that it can be
// referred to or released later.
type UploadResult struct {
	SlugID        string
	SourceVersion string
}

// Execute creates a new slug resource and uploads the compiled slug to it.
func (u *UploadCmd) Execute(ctx context.Context, o Outputter) (*UploadResult, error) {
	slug, err := u.Heroku.SlugCreate(ctx, u.Application, heroku.SlugCreateOpts{
		Checksum:                     heroku.String(u.Checksum),
		Commit:                       heroku.String(u.SourceVersion),
		Stack:                        heroku.String(u.Stack),
		BuildpackProvidedDescription: heroku.String(u.DetectedBuildpack),
		ProcessTypes:                 u.ProcessTypes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create slug: %w", err)
	}

	attempt := func() error {
		return UploadBlob(
			ctx,
			strings.ToUpper(slug.Blob.Method),
			slug.Blob.URL,
			u.Path,
		)
	}
	config := &backoff.ExponentialBackOff{
		InitialInterval:     100 * time.Millisecond,
		RandomizationFactor: 0.25,
		Multiplier:          2.0,
		MaxInterval:         5 * time.Second,
		MaxElapsedTime:      5 * time.Minute,
		Clock:               backoff.SystemClock,
	}
	if err := backoff.RetryNotify(attempt, config, func(err error, retryIn time.Duration) {
		fmt.Fprintf(o.ErrOrStderr(),
			"Error uploading slug retrying in %s: %s", retryIn, err)
	}); err != nil {
		return nil, fmt.Errorf("error uploading slug: %w", err)
	}

	return &UploadResult{
		SlugID:        slug.ID,
		SourceVersion: u.SourceVersion,
	}, nil
}

// UploadBlob uploads the file at the given path to the url using the given
// method.
func UploadBlob(ctx context.Context, method, url, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening blob file: %w", err)
	}

	fi, err := f.Stat()
	if err != nil {
		return fmt.Errorf("error stating blob file: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, f)
	if err != nil {
		return fmt.Errorf("error creating upload request: %w", err)
	}

	req.ContentLength = fi.Size()

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("error executing upload request: %w", err)
	}

	var body string
	defer response.Body.Close() // nolint:errcheck

	b, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	body = string(b)

	if response.StatusCode > 399 {
		return fmt.Errorf("error uploading slug response status %v: %v", response.Status, body)
	}

	return nil
}
