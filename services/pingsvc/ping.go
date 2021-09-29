package pingsvc

import (
	"context"

	proto "github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/store"
	"github.com/twitchtv/twirp"
)

var _ proto.Ping = (*service)(nil)

// service implements the Ping service.
type service struct {
	store store.Querier
}

// New builds returns an implementation of the ping.Ping service, which is
// primarily used to test/PoC.
func New(store store.Querier) proto.Ping {
	return &service{store: store}
}

// Echo echoes its given message.
func (s *service) Echo(_ context.Context, r *proto.EchoRequest) (*proto.EchoResponse, error) {
	return &proto.EchoResponse{Msg: r.Msg}, nil
}

// Boom returns an error.
func (s *service) Boom(_ context.Context, _ *proto.BoomRequest) (*proto.BoomResponse, error) {
	return nil, twirp.InternalError("boom")
}

// DatabaseHealth pings the database, checking if it is reachable.
func (s *service) DatabaseHealth(ctx context.Context, _ *proto.DatabaseHealthRequest) (*proto.DatabaseHealthResponse, error) {
	if err := s.store.Health(ctx); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}

	return &proto.DatabaseHealthResponse{}, nil
}
