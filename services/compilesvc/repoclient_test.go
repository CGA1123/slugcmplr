package compilesvc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/cga1123/slugcmplr/services/compilesvc"
	"github.com/google/go-github/v39/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GitHubDownloadURL(t *testing.T) {
	t.Parallel()

	var request *http.Request
	client, closer := fakeServer(func(w http.ResponseWriter, r *http.Request) {
		request = r

		w.Header().Add("Location", "https://test.com/archive")
		w.WriteHeader(302)
	})
	defer closer()

	c := compilesvc.NewGitHubRepoClient(github.NewClient(client))

	url, err := c.DownloadURL(context.Background(), "CGA1123", "slugcmplr", "abcd1234")
	require.NoError(t, err, "DownloadURL call should succeed")
	assert.Equal(t, "https://test.com/archive", url, "DownloadURL should return expected URL")

	require.NotNil(t, request, "A request should have been made to the server.")
	assert.Equal(t, "GET", request.Method, "Should make a GET request.")
	assert.Equal(t, "/repos/CGA1123/slugcmplr/tarball/abcd1234", request.URL.String(), "Should make a request to the tarball endpoint.")
	assert.Equal(t, "api.github.com", request.Host, "Should make a request to the GitHub API.")
}

func Test_GitHubConfigFile(t *testing.T) {
	t.Parallel()

	tableTOML := `[targets]
foo = 1`

	tests := []struct {
		Toml    string
		Error   error
		Targets []string
	}{
		{Toml: `targets = ["foo-app", "bar-app"]`, Targets: []string{"foo-app", "bar-app"}},
		{Toml: tableTOML, Error: errors.New("recovered panic during decoding")},
	}
	for _, tc := range tests {
		var request *http.Request
		client, closer := fakeServer(func(w http.ResponseWriter, r *http.Request) {
			request = r

			content := &github.RepositoryContent{Content: github.String(tc.Toml)}

			json.NewEncoder(w).Encode(content)
		})
		defer closer()

		c := compilesvc.NewGitHubRepoClient(github.NewClient(client))

		file, err := c.ConfigFile(context.Background(), "CGA1123", "slugcmplr", "abcd1234")

		assert.Equal(t, "GET", request.Method, "Should make a GET request.")
		assert.Equal(t, "api.github.com", request.Host, "Should make a request to the GitHub API.")
		assert.Equal(t, "/repos/CGA1123/slugcmplr/contents/.slugcmplr.toml?ref=abcd1234", request.URL.String(), "Should make a request to the contents endpoint.")

		if tc.Error != nil {
			assert.Error(t, err)
			assert.ErrorContains(t, err, tc.Error.Error())
		} else {
			require.NoError(t, err, "Should not error fetching config file.")

			assert.NotNil(t, file, "Should return a non-nil config file.")
			assert.NotNil(t, request, "Should have made a request to the server.")
			assert.Equal(t, []string{"foo-app", "bar-app"}, file.Targets, "Should return parsed targets.")
		}
	}
}
