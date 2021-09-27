package pingsvc_test

import (
	"context"
	"testing"

	"github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/services/pingsvc"
	"github.com/stretchr/testify/assert"
)

func Test_Echo(t *testing.T) {
	t.Parallel()

	svc := &pingsvc.Service{}

	response, err := svc.Echo(context.Background(), &ping.EchoRequest{Msg: "hello"})

	assert.NoError(t, err, "Echo should not error")
	assert.Equal(t, &ping.EchoResponse{Msg: "hello"}, response, "Response to echo request")
}
