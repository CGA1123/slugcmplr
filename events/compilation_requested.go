package events

import (
	"fmt"

	workers "github.com/digitalocean/go-workers2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"
)

// EventCompilationRequestedHandler handles `compilation_requested` messages,
// fanning out a job for each requested target.
func (i *impl) EventCompilationRequestedHandler(msg *workers.Msg) error {
	ctx := jobCtx(msg)
	span := trace.SpanFromContext(ctx)

	bidstr, err := msg.Args().String()
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "non-string arg"))

		return fmt.Errorf("expected arg to be string: %w", err)
	}

	bid, err := uuid.Parse(bidstr)
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "bad uuid"))

		return fmt.Errorf("expected arg to be uuid: %w", err)
	}

	br, err := i.s.GetBuildRequest(ctx, bid)
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "fetching build request"))

		return fmt.Errorf("error fetching build request: %s", err)
	}

	span.SetAttributes(
		attribute.String("build_request.id", bidstr),
		attribute.String("build_request.commit_sha", br.CommitSha),
		attribute.String("build_request.tree_sha", br.TreeSha),
		attribute.String("build_request.repository", br.Repository),
		attribute.String("build_request.owner", br.Owner),
	)

	g, gctx := errgroup.WithContext(ctx)
	for _, target := range br.Targets {
		t := target

		g.Go(func() error {
			return i.RequestTargetCompilation(gctx, bidstr, t)
		})
	}

	return g.Wait()
}
