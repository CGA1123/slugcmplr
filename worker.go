package slugcmplr

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/services/pingsvc"
	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

// WorkerCmd wraps up all the information required to start a slugcmplr worker
// process, consuming jobs from a given set of queues.
type WorkerCmd struct {
	Dequeuer queue.Dequeuer
	Queues   map[string]int
	Router   *mux.Router
}

// Execute starts a pool of goroutines to process jobs from the queue.
//
// This functions will block until it receives SIGINT/SIGTERM or the given
// context is cancelled.
func (w *WorkerCmd) Execute(ctx context.Context, _ Outputter) error {
	fn := queue.TwirpWorker(w.Router)
	pingsvc.Work(w.Router)

	shutdownC := make(chan os.Signal, 1)
	signal.Notify(shutdownC, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-shutdownC
		cancel()
	}()

	g, ctx := errgroup.WithContext(ctx)
	for name, workers := range w.Queues {
		for i := 0; i < workers; i++ {
			g.Go(consume(ctx, w.Dequeuer, name, fn))
		}
	}

	return g.Wait()
}

func consume(ctx context.Context, deq queue.Dequeuer, queue string, fn queue.Worker) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := deq.Deq(ctx, queue, fn); err != nil {
					log.Printf("error dequeueing from %v: %v", queue, err)

					// TODO: should this be exponential backoff to some limit?
					time.Sleep(time.Second)
				}
			}
		}
	}
}
