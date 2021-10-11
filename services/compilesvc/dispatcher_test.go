package compilesvc_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/cga1123/slugcmplr/services/compilesvc"
	heroku "github.com/heroku/heroku-go/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_HerokuDispatcher(t *testing.T) {
	t.Parallel()

	var request *http.Request
	var body []byte
	client, closer := fakeServer(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(500)
			return
		}

		body = b
		request = r

		dyno := &heroku.Dyno{}

		json.NewEncoder(w).Encode(dyno) // nolint:errcheck
	})
	defer closer()

	c := compilesvc.NewHerokuDispatcher(
		heroku.NewService(client),
		"compiler-app",
		"https://slugcmplr.test.io")

	err := c.Dispatch(context.Background(), "my-token")
	require.NoError(t, err, "Dispatching should not error.")

	assert.Equal(t, "POST", request.Method, "Should make a POST request.")
	assert.Equal(t, "api.heroku.com", request.Host, "Should make a call to the Heroku API.")
	assert.Equal(t, "/apps/compiler-app/dynos", request.URL.String(), "Should make a call to create a dyno.")

	expected, err := json.Marshal(&heroku.DynoCreateOpts{
		Attach:  heroku.Bool(false),
		Command: "slugcmplr receive",
		Env: map[string]string{
			"SLUGCMPLR_RECEIVE_TOKEN":   "my-token",
			"SLUGCMPLR_BASE_SERVER_URL": "https://slugcmplr.test.io",
		},
	})
	require.NoError(t, err)
	assert.Equal(t, string(expected), string(body), "Request to Heroku contains expected body.")
}

func Test_NullDispatcher(t *testing.T) {
	t.Parallel()

	err := compilesvc.NullDispatcher().Dispatch(context.Background(), "")
	assert.ErrorContains(t, err, "the null dispatcher does not dispatch any jobs")
}
