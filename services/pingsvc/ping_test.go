package pingsvc_test

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/queue"
	qstore "github.com/cga1123/slugcmplr/queue/store"
	"github.com/cga1123/slugcmplr/services/pingsvc"
	"github.com/cga1123/slugcmplr/store"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/twitchtv/twirp"
)

func client(s store.Querier, q queue.Enqueuer) (ping.Ping, func()) {
	m := mux.NewRouter()
	pingsvc.Route(m, s, q)

	server := httptest.NewServer(m)
	c := ping.NewPingJSONClient(server.URL, server.Client())

	return c, server.Close
}

func Test_Echo(t *testing.T) {
	t.Parallel()

	q := make(queue.InMemory, 0)
	svc, closer := client(&store.Memory{}, &q)
	defer closer()

	response, err := svc.Echo(context.Background(), &ping.EchoRequest{Msg: "hello"})

	assert.NoError(t, err, "Echo should not error.")
	assert.Equal(t, "hello", response.Msg, "Response to echo request.")
}

func Test_Boom(t *testing.T) {
	t.Parallel()

	q := make(queue.InMemory, 0)
	svc, closer := client(&store.Memory{}, &q)
	defer closer()

	_, err := svc.Boom(context.Background(), &ping.BoomRequest{})

	var terr twirp.Error
	assert.ErrorAs(t, err, &terr, "Boom should return a twirp.Error.")
	assert.Equal(t, twirp.Internal, terr.Code(), "The returned error should be of type twirp.Internal.")
	assert.Equal(t, "boom", terr.Msg(), "The error should boom.")
}

func Test_DatabaseHealth(t *testing.T) { // nolint:paralleltest
	cases := []struct{ err error }{{err: nil}, {err: errors.New("test error")}}
	for _, tc := range cases {
		q := make(queue.InMemory, 0)
		s := &store.Memory{}
		s.HealthErr = tc.err

		svc, closer := client(s, &q)
		defer closer()

		_, err := svc.DatabaseHealth(context.Background(), &ping.DatabaseHealthRequest{})

		if tc.err == nil {
			assert.NoError(t, err, "DatabaseHealth should not return an error.")
		} else {
			assert.Error(t, err, "DatabaseHealth should return an error.")
		}
	}
}

func Test_Queue(t *testing.T) {
	t.Parallel()

	q := make(queue.InMemory, 0)
	svc, closer := client(&store.Memory{}, &q)
	defer closer()

	r, err := svc.Queue(context.Background(), &ping.QueueRequest{Msg: "foo"})
	assert.NoError(t, err, "Queue should not error.")

	assert.Equal(t, 1, len(q), "Should have enqueued one message.")
	job := []qstore.QueuedJob(q)[0]
	assert.Equal(t, job.ID.String(), r.Jid, "The returned JID should match the enqueued JID.")
	assert.Equal(t, "default", job.QueueName, "The jobs should have been queued on default.")

	expected := fmt.Sprintf(`{"method":"/twirp/pingworker.Worker/Ping","base64_body":"%v"}`, base64.StdEncoding.EncodeToString([]byte(`{"msg":"foo"}`)))
	assert.Equal(t, expected, string(job.Data), "The jobs should have been queued with foo as data.")
}
