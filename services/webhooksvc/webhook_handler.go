package webhooksvc

import (
	"net/http"

	"github.com/cga1123/slugcmplr/proto/compileworker"
	"github.com/cga1123/slugcmplr/queue"
	"github.com/google/go-github/v39/github"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var _ http.Handler = (*service)(nil)

// Route registers the webhooksvc to handle requests to `POST /github/events`.
func Route(m *mux.Router, secret []byte, enq queue.Enqueuer) {
	svc := build(secret, enq)

	m.Handle("/github/events", svc).Methods(http.MethodPost)
}

type service struct {
	webhookSecret []byte
	tracer        trace.Tracer
	enq           compileworker.Compile
}

func build(secret []byte, enq queue.Enqueuer) http.Handler {
	return &service{
		webhookSecret: secret,
		enq:           compileworker.NewCompileJSONClient("", queue.TwirpEnqueuer(enq)),
		tracer:        otel.Tracer("github.com/CGA1123/slugcmplr/service/webhooksvc"),
	}
}

func (s *service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("webhook.type", github.WebHookType(r)),
		attribute.String("webhook.delivery_id", github.DeliveryID(r)),
	)

	payload, err := github.ValidatePayload(r, s.webhookSecret)
	if err != nil {
		span.SetAttributes(attribute.String("error.message", "invalid_signature"))
		http.Error(w, "invalid_signature", http.StatusUnauthorized)
		return
	}

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		span.SetAttributes(attribute.String("error.message", "invalid_payload"))
		http.Error(w, "invalid_payload", http.StatusUnprocessableEntity)
		return
	}

	push, ok := event.(*github.PushEvent)
	if !ok {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("skipped\n")) // nolint:errcheck
		return
	}

	if _, err := s.enq.TriggerForRepository(
		r.Context(),
		&compileworker.RepositoryInfo{
			EventId:    github.DeliveryID(r),
			CommitSha:  push.GetHeadCommit().GetSHA(),
			Owner:      push.GetRepo().GetOwner().GetLogin(),
			Repository: push.GetRepo().GetName(),
			Ref:        push.GetRef(),
		},
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("error_enqueuing\n")) // nolint:errcheck
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("processed\n")) // nolint:errcheck
}
