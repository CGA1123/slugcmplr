package events

import (
	"context"
	"math"
	"time"

	workers "github.com/digitalocean/go-workers2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func jobCtx(m *workers.Msg) context.Context {
	ctx, ok := m.Get("__context").Interface().(context.Context)
	if !ok {
		return context.Background()
	}

	return ctx
}

func otelMiddleware() workers.MiddlewareFunc {
	tracer := otel.Tracer("github.com/CGA1123/slugcmplr/events")

	return func(queue string, m *workers.Manager, n workers.JobFunc) workers.JobFunc {
		return func(m *workers.Msg) error {
			ctx := context.Background()

			enqueuedAt, _ := m.Get("enqueued_at").Float64()
			enqueueAtMs := int64(math.Floor(enqueuedAt * 1000))

			nowMs := time.Now().UnixMilli()
			waitedMs := nowMs - enqueueAtMs
			isRetry, _ := m.Get("retry").Bool()
			retryCount, _ := m.Get("retry_count").Int()
			opts := []trace.SpanOption{
				trace.WithSpanKind(trace.SpanKindConsumer),
				trace.WithAttributes(
					attribute.String("type", "job"),
					attribute.Int64("job.waited_ms", waitedMs),
					attribute.String("job.jid", m.Jid()),
					attribute.String("job.queue", queue),
					attribute.String("job.class", m.Class()),
					attribute.Bool("job.is_retry", isRetry),
					attribute.Int("job.retry_count", retryCount),
				),
			}

			ctx, span := tracer.Start(ctx, m.Class(), opts...)

			m.Set("__context", ctx)

			err := n(m)

			m.Del("__context")

			var attrs []attribute.KeyValue
			if err != nil {
				attrs = append(
					attrs,
					attribute.Bool("job.failed", true),
					attribute.String("job.error_message", err.Error()),
				)
			} else {
				attrs = append(
					attrs,
					attribute.Bool("job.failed", true),
				)
			}
			span.End(trace.WithAttributes(attrs...))

			return err
		}
	}
}

type obsProducer struct {
	p                 *workers.Producer
	t                 trace.Tracer
	runtimeAttributes []attribute.KeyValue
}

func newObsProducer(p *workers.Producer) *obsProducer {
	return &obsProducer{
		p: p,
		t: otel.Tracer("github.com/CGA1123/slugcmplr/events"),
	}
}

func (o *obsProducer) Enqueue(ctx context.Context, queue, event string, args interface{}) (string, error) {
	_, span := o.t.Start(ctx, event,
		trace.WithAttributes(o.runtimeAttributes...),
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("job.queue", queue),
			attribute.String("job.class", event),
		),
	)
	defer span.End()

	jid, err := o.p.Enqueue(queue, event, args)
	span.SetAttributes(attribute.String("job.id", jid))

	if err != nil {
		span.SetAttributes(
			attribute.Bool("job.failed", true),
			attribute.String("job.error_message", err.Error()),
		)
	}

	return jid, err
}
