package pingsvc_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/services/pingsvc"
	"github.com/cga1123/slugcmplr/store"
	"github.com/stretchr/testify/assert"
	"github.com/twitchtv/twirp"
)

func Test_Echo(t *testing.T) {
	t.Parallel()

	svc := pingsvc.New(&store.Memory{})

	response, err := svc.Echo(context.Background(), &ping.EchoRequest{Msg: "hello"})

	assert.NoError(t, err, "Echo should not error.")
	assert.Equal(t, &ping.EchoResponse{Msg: "hello"}, response, "Response to echo request.")
}

func Test_Boom(t *testing.T) {
	t.Parallel()

	svc := pingsvc.New(&store.Memory{})

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

		svc := pingsvc.New(s)

		res, err := svc.DatabaseHealth(context.Background(), &ping.DatabaseHealthRequest{})

		if tc.err == nil {
			assert.NoError(t, err, "DatabaseHealth should not return an error.")
			assert.Equal(t, &ping.DatabaseHealthResponse{}, res, "DatabaseHealth should return an appropriate response.")
		} else {
			assert.Error(t, err, "DatabaseHealth should return an error.")
		}
	}
}
