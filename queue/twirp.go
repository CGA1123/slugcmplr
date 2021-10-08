package queue

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/cga1123/slugcmplr/queue/store"
)

type message struct {
	Method     string `json:"method"`
	Base64Body string `json:"base64_body"`
}

// TwirpWorker creates a new twirp based Worker.
func TwirpWorker(h http.Handler) Worker {
	return &worker{handler: h}
}

type worker struct {
	handler http.Handler
}

func (w *worker) Do(ctx context.Context, j store.QueuedJob) error {
	var msg message
	if err := json.Unmarshal(j.Data, &msg); err != nil {
		return err
	}

	body, err := base64.StdEncoding.DecodeString(msg.Base64Body)
	if err != nil {
		return err
	}

	req, err := newRequest(ctx, msg.Method, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req = req.WithContext(ctx)
	recorder := httptest.NewRecorder()

	w.handler.ServeHTTP(recorder, req)

	resp := recorder.Result()
	if resp.StatusCode != 200 {
		return errorFromResponse(resp)
	}

	return nil
}

// TODO: how to decide if an error is retryable?
func (w *worker) Retryable(_ store.QueuedJob, _ error) (bool, time.Duration) {
	return false, time.Duration(0)
}

// HTTPClient is the interface used to send HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// TwirpEnqueuer builds a HTTPClient which can be used to build a JSON Twirp
// client that enqueues to the given enqueuer.
func TwirpEnqueuer(enq Enqueuer) HTTPClient {
	return &httpEnqueuer{enq: enq}
}

type httpEnqueuer struct {
	enq Enqueuer
}

func (e *httpEnqueuer) Do(r *http.Request) (*http.Response, error) {
	method := r.URL.Path
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close() // nolint:errcheck

	msg, err := json.Marshal(message{Method: method, Base64Body: base64.StdEncoding.EncodeToString(body)})
	if err != nil {
		return nil, fmt.Errorf("error marshaling payload: %w", err)
	}

	// TODO: how to enable selecting the queue name?
	id, err := e.enq.Enq(r.Context(), "default", msg)
	if err != nil {
		return nil, fmt.Errorf("error enqueueing job: %w", err)
	}

	resp := fmt.Sprintf(`{"jid":"%v"}`, id.String())
	return &http.Response{
		Status:        "200 OK",
		StatusCode:    200,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Body:          ioutil.NopCloser(bytes.NewBufferString(resp)),
		ContentLength: int64(len(resp)),
		Request:       r,
		Header:        make(http.Header),
	}, nil
}
