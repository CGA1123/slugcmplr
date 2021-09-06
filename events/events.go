package events

import (
	"context"
	"fmt"

	"github.com/cga1123/slugcmplr/dispatch"
	"github.com/cga1123/slugcmplr/store"
	workers "github.com/digitalocean/go-workers2"
)

type event int

const (
	eventUnknown event = iota
	eventPing
	eventCompilationRequested
	eventTargetCompilationRequested
)

func eventFromString(s string) event {
	e, ok := map[string]event{
		"ping":                         eventPing,
		"compilation_requested":        eventCompilationRequested,
		"target_compilation_requested": eventTargetCompilationRequested,
	}[s]
	if !ok {
		return eventUnknown
	}

	return e
}

func (e event) ToString() string {
	return map[event]string{
		eventPing:                       "ping",
		eventCompilationRequested:       "compilation_requested",
		eventTargetCompilationRequested: "target_compilation_requested",
	}[e]
}

type queue int

const (
	queueDefault queue = iota
	queueEvents
)

func (q queue) ToString() string {
	return map[queue]string{
		queueDefault: "default",
		queueEvents:  "events",
	}[q]
}

// Events represents all enqueueable events.
type Events interface {
	Ping(context.Context, string) error
	RequestCompilation(context.Context, string) error
	RequestTargetCompilation(context.Context, string, string) error
}

type impl struct {
	p *obsProducer
	s store.Querier
	d dispatch.Dispatcher
}

// New builds a new workers backed Events enqueuer.
func New(m *workers.Manager, s store.Querier, d dispatch.Dispatcher) Events {
	i := &impl{p: newObsProducer(m.Producer()), s: s, d: d}

	middlewares := workers.DefaultMiddlewares().Append(
		otelMiddleware(),
	)

	m.AddWorker(queueDefault.ToString(), 20, i.handler, middlewares...)
	m.AddWorker(queueEvents.ToString(), 20, i.handler, middlewares...)

	return i
}

func (i *impl) handler(msg *workers.Msg) error {
	e := eventFromString(msg.Class())
	var h workers.JobFunc

	switch e {
	case eventUnknown:
		h = i.EventUnknownHandler
	case eventPing:
		h = i.EventPingHandler
	case eventCompilationRequested:
		h = i.EventCompilationRequestedHandler
	case eventTargetCompilationRequested:
		h = i.EventTargetCompilationRequestedHandler
	}

	return h(msg)
}

func (i *impl) RequestCompilation(ctx context.Context, bid string) error {
	_, err := i.p.Enqueue(
		ctx,
		queueEvents.ToString(),
		eventCompilationRequested.ToString(),
		bid,
	)

	return err
}

func (i *impl) RequestTargetCompilation(ctx context.Context, bid, target string) error {
	_, err := i.p.Enqueue(
		ctx,
		queueEvents.ToString(),
		eventTargetCompilationRequested.ToString(),
		[]string{bid, target},
	)

	return err
}

func (i *impl) Ping(ctx context.Context, pong string) error {
	_, err := i.p.Enqueue(ctx, queueDefault.ToString(), eventPing.ToString(), pong)

	return err
}

func (i *impl) EventUnknownHandler(msg *workers.Msg) error {
	return fmt.Errorf("event %v unknown", msg.Class())
}
