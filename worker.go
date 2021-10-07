package slugcmplr

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cga1123/slugcmplr/queue"
	"golang.org/x/sync/errgroup"
)

// WorkerCmd wraps up all the information required to start a slugcmplr worker
// process, consuming jobs from a given set of queues.
type WorkerCmd struct {
	Dequeuer queue.Dequeuer
}

func (w *WorkerCmd) Execute(ctx context.Context, _ Outputter) error {
	shutdownC := make(chan os.Signal, 1)
	signal.Notify(shutdownC, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-shutdownC
		cancel()
	}()

	g, ctx := errgroup.WithContext(ctx)
	queues := map[string]queue.Worker{}

	for queue, worker := range queues {
		g.Go(doWork(ctx, w.Dequeuer, queue, worker))
	}

	return g.Wait()
}

func doWork(ctx context.Context, deq queue.Dequeuer, queue string, worker queue.Worker) func() error {
	return func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				if err := deq.Deq(ctx, queue, worker); err != nil {
					log.Printf("error dequeueing from %v: %v", queue, err)

					// TODO: should this be exponential backoff to some limit?
					time.Sleep(time.Second)
				}
			}
		}
	}
}
