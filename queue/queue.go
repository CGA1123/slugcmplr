package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/cga1123/slugcmplr/obs"
	"github.com/cga1123/slugcmplr/queue/store"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Enqueuer describes the interface for enqueueing a job to a queue.
type Enqueuer interface {
	Enq(context.Context, string, []byte, ...JobOptions) (uuid.UUID, error)
}

// Dequeuer describes the interface for dequeueing a job from a queue.
type Dequeuer interface {
	Deq(context.Context, string, Worker) error
}

// Queue describes the interface for a queue, combining an Enqueuer and
// Dequeuer.
type Queue interface {
	Enqueuer
	Dequeuer
}

// BackoffFunc computes the backoff for a retry given the current attempt number.
type BackoffFunc func(int) time.Duration

// ConstantBackoff implements a BackoffFunc which returns a constant value.
func ConstantBackoff(d time.Duration) BackoffFunc {
	return func(_ int) time.Duration {
		return d
	}
}

// Worker describes the expected interface for a worker, with configurable
// retry behaviour.
type Worker interface {
	Do(context.Context, store.QueuedJob) error
	Retryable(store.QueuedJob, error) (bool, time.Duration)
}

// RetryWorker implements a bounded retriers with the given backoff strategy.
func RetryWorker(max int, backoff BackoffFunc, f func(context.Context, store.QueuedJob) error) Worker {
	return &retryWorker{
		max:     max,
		f:       f,
		backoff: backoff,
	}
}

type retryWorker struct {
	max     int
	f       func(context.Context, store.QueuedJob) error
	backoff BackoffFunc
}

// Do executes the given functions for a job.
func (r *retryWorker) Do(ctx context.Context, j store.QueuedJob) error {
	return r.f(ctx, j)
}

// Retryable will check if the current job is within its allowed attempts.
func (r *retryWorker) Retryable(j store.QueuedJob, _ error) (bool, time.Duration) {
	if j.Attempt >= int32(r.max) {
		return false, time.Duration(0)
	}

	return true, r.backoff(int(j.Attempt))
}

// NoRetryWorker creates a worker with no retries.
func NoRetryWorker(f func(context.Context, store.QueuedJob) error) Worker {
	return &noRetryWorker{f: f}
}

type noRetryWorker struct {
	f func(context.Context, store.QueuedJob) error
}

// Do executes the given function.
func (n *noRetryWorker) Do(ctx context.Context, j store.QueuedJob) error {
	return n.f(ctx, j)
}

// Retryable always returns false.
func (n *noRetryWorker) Retryable(_ store.QueuedJob, _ error) (bool, time.Duration) {
	return false, time.Duration(0)
}

// JobOptions are functions which may mutate the EnqueueParams for a job away
// from their defaults.
type JobOptions func(*store.EnqueueParams)

// Attempt sets a custom attempt, affecting retries. Default is 0.
func Attempt(i int) JobOptions {
	return func(p *store.EnqueueParams) {
		p.Attempt = int32(i)
	}
}

// ScheduledAt sets a custom schedule time. Default is the time of enqueueing.
func ScheduledAt(t time.Time) JobOptions {
	return func(p *store.EnqueueParams) {
		p.ScheduledAt = t
	}
}

// PGQueue contains the state required for the PG backed queue implementation.
type PGQueue struct {
	name     string
	db       *pgxpool.Pool
	enqStore store.Querier
	fn       Worker
	tracer   trace.Tracer
}

// New creates a new Queue.
func New(db *pgxpool.Pool) *PGQueue {
	return &PGQueue{
		db:       db,
		enqStore: store.New(obs.NewDB(db)),
		tracer:   otel.Tracer("github.com/CGA1123/slugcmplr/queue"),
	}
}

// Enq enqueues a single job to the queue.
func (q *PGQueue) Enq(ctx context.Context, queue string, data []byte, opts ...JobOptions) (uuid.UUID, error) {
	ctx, span := q.tracer.Start(ctx, "enqueue",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("postgres"),
			semconv.MessagingDestinationKey.String(q.name),
			semconv.MessagingDestinationKindQueue,
		),
	)
	defer span.End()

	now := time.Now()
	params := store.EnqueueParams{
		QueueName:   queue,
		Data:        data,
		ScheduledAt: now,
		Attempt:     0}
	for _, opt := range opts {
		opt(&params)
	}

	jid, err := q.enqStore.Enqueue(ctx, params)
	span.SetAttributes(
		semconv.MessagingMessageIDKey.String(jid.String()),
		semconv.MessagingMessagePayloadSizeBytesKey.Int(len(params.Data)),
		attribute.Int64("messaging.delay_ms", int64(params.ScheduledAt.Sub(now)/time.Millisecond)),
		attribute.Int("messaging.delivery_attempt", int(params.Attempt)),
	)
	if err != nil {
		span.SetStatus(codes.Error, fmt.Sprintf("error enqueueing: %v", err))
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return jid, err
}

// Deq dequeues a single job for the queue. The worker may retry based on it's
// settings, if retries become exhausted, the job will be moved to the dead
// letter queue.
func (q *PGQueue) Deq(ctx context.Context, queue string, worker Worker) error {
	ctx, span := q.tracer.Start(ctx, "dequeue",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			semconv.MessagingSystemKey.String("postgres"),
			semconv.MessagingDestinationKey.String(queue),
			semconv.MessagingDestinationKindQueue,
		))
	defer span.End()

	tx, err := q.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx) // nolint:errcheck

	s := store.New(obs.NewDB(tx))
	j, err := s.Dequeue(ctx, q.name)
	if err != nil {
		return fmt.Errorf("failed to dequeue a job: %w", err)
	}

	span.SetAttributes(
		semconv.MessagingMessageIDKey.String(j.ID.String()),
		semconv.MessagingMessagePayloadSizeBytesKey.Int(len(j.Data)),
		attribute.Int64("messaging.delay_ms", int64(time.Since(j.ScheduledAt)/time.Millisecond)),
		attribute.Int("messaging.delivery_attempt", int(j.Attempt)),
	)

	if err := worker.Do(ctx, j); err != nil {
		span.SetStatus(codes.Error, fmt.Sprintf("error processing job: %v", err))

		retry, backoff := q.fn.Retryable(j, err)

		if retry {
			nj := store.EnqueueParams{
				QueueName:   q.name,
				Data:        j.Data,
				ScheduledAt: time.Now().Add(backoff),
				Attempt:     j.Attempt + 1}
			if _, err := s.Enqueue(ctx, nj); err != nil {
				span.SetStatus(codes.Error, fmt.Sprintf("error enqueueing retry: %v", err))

				return fmt.Errorf("failed to re-enqueue job: %w", err)
			}
		} else {
			if err := s.DeadLetter(ctx, store.DeadLetterParams(j)); err != nil {
				span.SetStatus(codes.Error, fmt.Sprintf("error moving to deadletter: %v", err))

				return fmt.Errorf("failed to dead-letter job: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		span.SetStatus(codes.Error, fmt.Sprintf("error committing processed job: %v", err))

		return fmt.Errorf("failed to commit to database: %w", err)
	}

	span.SetStatus(codes.Ok, "")

	return nil
}
