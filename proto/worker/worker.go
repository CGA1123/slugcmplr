package worker

import (
	bytes "bytes"
	context "context"
	"encoding/base64"
	json "encoding/json"
	fmt "fmt"
	io "io"
	ioutil "io/ioutil"
	http "net/http"
	"net/http/httptest"
	"time"

	"github.com/cga1123/slugcmplr/queue"
	qstore "github.com/cga1123/slugcmplr/queue/store"
	"github.com/cga1123/slugcmplr/services"
	twirp "github.com/twitchtv/twirp"
)

type Message struct {
	Method     string `json:"method"`
	Base64Body string `json:"base64_body"`
}

func NewEnqueuer(enq queue.Enqueuer) Worker {
	return NewWorkerJSONClient("", &httpEnqueuer{enq: enq})
}

type httpEnqueuer struct {
	enq queue.Enqueuer
}

func (e *httpEnqueuer) Do(r *http.Request) (*http.Response, error) {
	method := r.URL.Path
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}
	defer r.Body.Close()

	msg, err := json.Marshal(Message{Method: method, Base64Body: base64.StdEncoding.EncodeToString(body)})
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
		Header:        make(http.Header, 0),
	}, nil
}

func NewWorker(svc Worker) queue.Worker {
	return &worker{
		handler: NewWorkerServer(svc, twirp.WithServerInterceptors(services.TwirpOtelInterceptor())),
	}
}

type worker struct {
	handler http.Handler
}

func (w *worker) Do(ctx context.Context, j qstore.QueuedJob) error {
	var msg Message
	if err := json.Unmarshal(j.Data, &msg); err != nil {
		return err
	}

	body, err := base64.StdEncoding.DecodeString(msg.Base64Body)
	if err != nil {
		return err
	}

	req, err := newRequest(ctx, msg.Method, bytes.NewBuffer(body), "application/json")
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
func (w *worker) Retryable(j qstore.QueuedJob, err error) (bool, time.Duration) {
	return false, time.Duration(0)
}
