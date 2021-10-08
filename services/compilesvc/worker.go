package compilesvc

import (
	"context"
	"fmt"

	"github.com/cga1123/slugcmplr/obs"
	"github.com/cga1123/slugcmplr/proto/compileworker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/gorilla/mux"
	"github.com/twitchtv/twirp"
)

var _ compileworker.Compile = (*worker)(nil)

// Work registers the jobs for the compile service with the worker.
func Work(m *mux.Router, enq queue.Enqueuer) {
	svc := &worker{
		compileEnq: compileworker.NewCompileJSONClient("", queue.TwirpEnqueuer(enq)),
	}

	m.PathPrefix("").Handler(compileworker.NewCompileServer(svc, twirp.WithServerInterceptors(obs.TwirpOtelInterceptor())))
}

type worker struct {
	compileEnq compileworker.Compile
}

func (w *worker) TriggerForRepository(context.Context, *compileworker.RepositoryInfo) (*compileworker.JobResponse, error) {
	return nil, fmt.Errorf("nyi")
}

func (w *worker) TriggerForTarget(context.Context, *compileworker.TargetInfo) (*compileworker.JobResponse, error) {
	return nil, fmt.Errorf("nyi")
}
