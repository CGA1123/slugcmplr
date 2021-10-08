package pingsvc

import (
	"context"

	"github.com/cga1123/slugcmplr/proto/ping"
	"github.com/cga1123/slugcmplr/proto/worker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/services"
	"github.com/cga1123/slugcmplr/store"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
)

var _ ping.Ping = (*service)(nil)

// service implements the Ping service.
type service struct {
	store store.Querier
	queue worker.Worker
}

// Route registers the twirp pingsvc onto the given router.
func Route(m *mux.Router, store store.Querier, enq queue.Enqueuer) {
	svc := ping.NewPingServer(build(store, enq), twirp.WithServerInterceptors(services.TwirpOtelInterceptor()))

	m.PathPrefix(ping.PingPathPrefix).Handler(svc)
}

func build(store store.Querier, queue queue.Enqueuer) ping.Ping {
	return &service{store: store, queue: worker.NewEnqueuer(queue)}
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

func (s *service) Queue(ctx context.Context, r *ping.QueueRequest) (*ping.QueueResponse, error) {
	id, err := s.queue.Ping(ctx, &worker.PingRequest{Msg: r.Msg})
	if err != nil {
		return nil, twirp.InternalErrorWith(err)
	}

	return &ping.QueueResponse{Jid: id.Jid}, nil
}
