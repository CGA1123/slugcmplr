package queue_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/cga1123/slugcmplr/proto/pingworker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type svc struct {
	msg []string
}

func (s *svc) Ping(_ context.Context, r *pingworker.PingRequest) (*pingworker.JobResponse, error) {
	s.msg = append(s.msg, r.Msg)

	return &pingworker.JobResponse{}, nil
}

func Test_Worker(t *testing.T) {
	t.Parallel()

	q := make(queue.InMemory, 0)
	enq := pingworker.NewWorkerJSONClient("", queue.TwirpEnqueuer(&q))

	r, err := enq.Ping(context.Background(), &pingworker.PingRequest{Msg: "test"})
	require.NoError(t, err, "Should enqueue successfully.")

	assert.Equal(t, 1, len(q), "Should have enqueued one job.")

	job := q[0]
	assert.Equal(t, job.ID.String(), r.Jid, "The returned JID should match the enqueued JID.")
	assert.Equal(t, "default", job.QueueName, "The jobs should have been queued on default.")

	expected := fmt.Sprintf(`{"method":"/twirp/pingworker.Worker/Ping","base64_body":"%v"}`, base64.StdEncoding.EncodeToString([]byte(`{"msg":"test"}`)))
	assert.Equal(t, expected, string(job.Data), "The jobs should have been queued with foo as data.")

	s := &svc{msg: make([]string, 0)}
	worker := queue.TwirpWorker(pingworker.NewWorkerServer(s))

	require.NoError(t, worker.Do(context.Background(), q[0]), "Worker should process job successfully.")

	assert.Equal(t, 1, len(s.msg), "Should have process one message.")
	assert.Equal(t, "test", s.msg[0], "Should have processed the expected message.")
}
