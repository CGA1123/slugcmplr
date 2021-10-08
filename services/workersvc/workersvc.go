package workersvc

import (
	"context"
	"log"

	"github.com/cga1123/slugcmplr/proto/worker"
)

var _ worker.Worker = (*service)(nil)

func New() worker.Worker {
	return &service{}
}

type service struct {
}

func (s *service) Ping(_ context.Context, r *worker.PingRequest) (*worker.JobResponse, error) {
	log.Printf("ping job: %v", r.Msg)

	return &worker.JobResponse{}, nil
}
