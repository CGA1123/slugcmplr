package events

import (
	"log"

	workers "github.com/digitalocean/go-workers2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// EventPingHandler handles `ping` messages, printing out its string argument.
func (i *impl) EventPingHandler(msg *workers.Msg) error {
	span := trace.SpanFromContext(jobCtx(msg))

	s, err := msg.Args().String()
	if err != nil {
		return err
	}

	span.SetAttributes(attribute.String("ping", s))

	log.Printf("pong: %v", s)

	return nil
}
