package webhooksvc_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/services/webhooksvc"
	"github.com/google/go-github/v39/github"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func currentDir(t *testing.T) string {
	_, file, _, ok := runtime.Caller(1)
	require.True(t, ok, "Should be able to get current file")

	return filepath.Dir(file)
}

func Test_Handler(t *testing.T) {
	t.Parallel()

	dir := currentDir(t)
	pushPayload, err := ioutil.ReadFile(filepath.Join(dir, "push_event.json"))
	require.NoError(t, err, "Should read fixture file successfully")

	starPayload, err := ioutil.ReadFile(filepath.Join(dir, "star_event.json"))
	require.NoError(t, err, "Should read fixture file successfully")

	cases := []struct {
		description    string
		payload        []byte
		secret         []byte
		signature      string
		event          string
		deliveryID     string
		expectedBody   string
		expectedStatus int
	}{
		{
			description:    "with a valid payload",
			payload:        pushPayload,
			secret:         []byte("test_secret"),
			signature:      signPayload(pushPayload, []byte("test_secret")),
			event:          "push",
			deliveryID:     uuid.NewString(),
			expectedBody:   "processed\n",
			expectedStatus: http.StatusAccepted,
		},
		{
			description:    "with an invalid signature",
			payload:        pushPayload,
			secret:         []byte("test_secret"),
			signature:      signPayload(pushPayload, []byte("not_test_secret")),
			event:          "push",
			deliveryID:     uuid.NewString(),
			expectedBody:   "invalid_signature\n",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			description:    "with valid but unregistered event",
			payload:        starPayload,
			secret:         []byte("test_secret"),
			signature:      signPayload(starPayload, []byte("test_secret")),
			event:          "star",
			deliveryID:     uuid.NewString(),
			expectedBody:   "skipped\n",
			expectedStatus: http.StatusAccepted,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			m := mux.NewRouter()

			q := make(queue.InMemory, 0)
			webhooksvc.Route(m, tc.secret, &q)

			body := bytes.NewReader(tc.payload)
			r, err := http.NewRequest(http.MethodPost, "/github/events", body)
			require.NoError(t, err, "Request should be created without error.")

			r.Header.Add(github.SHA256SignatureHeader, tc.signature)
			r.Header.Add(github.EventTypeHeader, tc.event)
			r.Header.Add(github.DeliveryIDHeader, tc.deliveryID)
			r.Header.Add("Content-Type", "application/json")

			m.ServeHTTP(recorder, r)

			result := recorder.Result()
			defer result.Body.Close() // nolint:errcheck

			response, err := ioutil.ReadAll(result.Body)
			require.NoError(t, err, "Response body should be read cleanly")

			assert.Equal(t, tc.expectedStatus, result.StatusCode, "The expected status code should be returned")
			assert.Equal(t, tc.expectedBody, string(response), "The expected response body should be returned")

			if tc.expectedBody == "processed\n" {
				assert.Equal(t, 1, len(q), "Should have enqueued one job.")
			} else {
				assert.Equal(t, 0, len(q), "Should not have enqueued any job.")
			}
		})
	}
}

func signPayload(payload, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload) // nolint:errcheck
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
