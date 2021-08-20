// package slugignore implements Heroku-like .slugignore functionality for a
// given directory.
//
// Heroku's .slugignore format treats all non-empty and non comment lines
// (comment lines are those beginning with a # characted) as Ruby Dir globs,
package slugignore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// SlugIgnore is the interface for a parsed .slugignore file
//
// A given SlugIgnore is only applicable to the directory which contains the
// .slugignore file, at the time at which it was parsed.
type SlugIgnore interface {
	IsIgnored(path string) bool
}

// ForDirectory parses the .slugignore for a given directory.
//
// If there is no .slugignore file found, it returns a SlugIgnore which always
// returns false when IsIgnored is called.
func ForDirectory(dir string) (SlugIgnore, error) {
	f, err := os.Open(filepath.Join(dir, ".slugignore"))
	if err != nil {
		if err == os.ErrNotExist {
			return &nullSlugIgnore{}, nil
		}
	}
	defer f.Close()

	s := bufio.NewScanner(bufio.NewReader(f))
	globs := []string{}

	for s.Scan() {
		line := s.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		if strings.TrimSpace(line) == "" {
			continue
		}

		trimmed := strings.TrimPrefix(line, "/")
		if strings.HasPrefix(line, "/") {
			globs = append(globs, trimmed)
		} else {
			globs = append(
				globs,
				trimmed,
				filepath.Join("**", trimmed),
			)
		}
	}
	if err := s.Err(); err != nil {
		return nil, fmt.Errorf("error parsing .slugignore: %w", err)
	}

	ignored := map[string]struct{}{}
	for _, glob := range globs {
		if !doublestar.ValidatePattern(glob) {
			return nil, fmt.Errorf("slugignore pattern is malformed: %v", glob)
		}

		matches, err := doublestar.Glob(os.DirFS(dir), glob)
		if err != nil {
			return nil, fmt.Errorf("error expanding glob %v: %w", glob, err)
		}

		for _, match := range matches {
			ignored[match] = struct{}{}
		}
	}

	return cache(ignored), nil
}

type nullSlugIgnore struct{}

func (*nullSlugIgnore) IsIgnored(path string) bool {
	return false
}

type cache map[string]struct{}

func (c cache) IsIgnored(path string) bool {
	path = strings.TrimPrefix(path, "/")

	for {
		if _, ok := c[path]; ok {
			return true
		}

		path = filepath.Join(path, "..")

		if path == "." {
			break
		}
	}

	return false
}
