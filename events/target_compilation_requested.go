package events

import (
	"crypto/rand"
	"fmt"

	"github.com/cga1123/slugcmplr/store"
	workers "github.com/digitalocean/go-workers2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// EventTargetCompilationRequestedHandler handles
// `target_compilation_requested` messages, kicking off a compilation worker
// for the given target.
func (i *impl) EventTargetCompilationRequestedHandler(msg *workers.Msg) error {
	ctx := jobCtx(msg)
	span := trace.SpanFromContext(ctx)

	args, err := msg.Args().StringArray()
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "non-string array arg"))

		return fmt.Errorf("expected arg to be string: %w", err)
	}

	bidstr, target := args[0], args[1]
	bid, err := uuid.Parse(bidstr)
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "non uuid id arg"))
	}

	br, err := i.s.GetBuildRequest(ctx, bid)
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "fetching build request"))

		return fmt.Errorf("error fetching build request: %s", err)
	}
	span.SetAttributes(
		attribute.String("build_request.id", bidstr),
		attribute.String("build_request.target", target),
		attribute.String("build_request.commit_sha", br.CommitSha),
		attribute.String("build_request.tree_sha", br.TreeSha),
		attribute.String("build_request.repository", br.Repository),
		attribute.String("build_request.owner", br.Owner),
	)

	token, err := token()
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "generating token"))

		return err
	}

	result, err := i.s.CreateReceiveToken(ctx, store.CreateReceiveTokenParams{
		BuildRequestID:     bid,
		BuildRequestTarget: target,
		Token:              token,
	})
	if err != nil {
		span.SetAttributes(attribute.String("job.error_reason", "storing token"))

		return err
	}

	span.SetAttributes(attribute.String("job.token_id", result.String()))

	return i.d.Dispatch(ctx, token)
}

func token() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("error creating token: %w", err)
	}

	return fmt.Sprintf("%x", b), nil
}
