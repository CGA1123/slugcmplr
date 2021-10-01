package pingsvc_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/services/pingsvc"
	"github.com/cga1123/slugcmplr/store"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/twitchtv/twirp"
)

func client(s store.Querier) (ping.Ping, func()) {
	m := mux.NewRouter()
	pingsvc.Route(m, s)

	server := httptest.NewServer(m)
	c := ping.NewPingJSONClient(server.URL, server.Client())

	return c, server.Close
}

func Test_Echo(t *testing.T) {
	t.Parallel()

	svc, closer := client(&store.Memory{})
	defer closer()

	response, err := svc.Echo(context.Background(), &ping.EchoRequest{Msg: "hello"})

	assert.NoError(t, err, "Echo should not error.")
	assert.Equal(t, "hello", response.Msg, "Response to echo request.")
}

func Test_Boom(t *testing.T) {
	t.Parallel()

	svc, closer := client(&store.Memory{})
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
		s := &store.Memory{}
		s.HealthErr = tc.err

		svc, closer := client(s)
		defer closer()

		_, err := svc.DatabaseHealth(context.Background(), &ping.DatabaseHealthRequest{})

		if tc.err == nil {
			assert.NoError(t, err, "DatabaseHealth should not return an error.")
		} else {
			assert.Error(t, err, "DatabaseHealth should return an error.")
		}
	}
}
