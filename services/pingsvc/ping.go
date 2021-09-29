package pingsvc

import (
	"context"

	proto "github.com/cga1123/slugcmplr/proto/ping"
	"github.com/twitchtv/twirp"
)

var _ proto.Ping = (*Service)(nil)

// Service implements the Ping service.
type Service struct{}

// Echo echoes its given message.
func (s *Service) Echo(_ context.Context, r *proto.EchoRequest) (*proto.EchoResponse, error) {
	return &proto.EchoResponse{Msg: r.Msg}, nil
}

// Boom returns an error.
func (s *Service) Boom(_ context.Context, _ *proto.BoomRequest) (*proto.BoomResponse, error) {
	return nil, twirp.InternalError("boom")
}
