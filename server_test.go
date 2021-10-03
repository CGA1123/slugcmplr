package slugcmplr_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cga1123/slugcmplr"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testHandler(router *mux.Router, r *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, r)

	return recorder
}

func TestServer_Root(t *testing.T) {
	t.Parallel()

	s := &slugcmplr.ServerCmd{Environment: "test"}
	req, err := http.NewRequest("GET", "/", nil)
	require.NoError(t, err, "Request should be built successfully")

	res := testHandler(s.Router(), req).Result()
	defer res.Body.Close() // nolint:errcheck

	assert.Equal(t, 302, res.StatusCode, "Response should be a redirect")
	assert.Equal(t, "https://imgs.xkcd.com/comics/compiling.png", res.Header.Get("Location"))
}

func TestServer_BadPath(t *testing.T) {
	t.Parallel()

	s := &slugcmplr.ServerCmd{Environment: "test"}
	req, err := http.NewRequest("GET", "/not-a-reasonable-path", nil)
	require.NoError(t, err, "Request should be built successfully")

	res := testHandler(s.Router(), req).Result()
	defer res.Body.Close() // nolint:errcheck

	assert.Equal(t, 404, res.StatusCode, "Response should be a not found")
}
