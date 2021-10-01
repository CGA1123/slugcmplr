package pingsvc

import (
	"context"

	"github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/services"
	"github.com/cga1123/slugcmplr/store"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
)

var _ ping.Ping = (*service)(nil)

// service implements the Ping service.
type service struct {
	store store.Querier
}

// Route registers the twirp pingsvc onto the given router.
func Route(m *mux.Router, store store.Querier) {
	svc := ping.NewPingServer(build(store), twirp.WithServerInterceptors(services.TwirpOtelInterceptor()))

	m.PathPrefix(ping.PingPathPrefix).Handler(svc)
}

func build(store store.Querier) ping.Ping {
	return &service{store: store}
}

// Echo echoes its given message.
func (s *service) Echo(_ context.Context, r *ping.EchoRequest) (*ping.EchoResponse, error) {
	return &ping.EchoResponse{Msg: r.Msg}, nil
}

// Boom returns an error.
func (s *service) Boom(_ context.Context, _ *ping.BoomRequest) (*ping.BoomResponse, error) {
	return nil, twirp.InternalError("boom")
}

// DatabaseHealth pings the database, checking if it is reachable.
func (s *service) DatabaseHealth(ctx context.Context, _ *ping.DatabaseHealthRequest) (*ping.DatabaseHealthResponse, error) {
	if err := s.store.Health(ctx); err != nil {
		return nil, twirp.InternalErrorWith(err)
	}

	return &ping.DatabaseHealthResponse{}, nil
}
