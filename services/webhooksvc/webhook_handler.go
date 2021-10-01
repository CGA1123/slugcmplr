package webhooksvc

import (
	"fmt"
	"net/http"

	"github.com/google/go-github/v39/github"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var _ http.Handler = (*service)(nil)

// Route registers the webhooksvc to handle requests to `POST /github/events`.
func Route(m *mux.Router, secret []byte) {
	svc := build(secret)

	m.Handle("/github/events", svc).Methods(http.MethodPost)
}

type service struct {
	webhookSecret []byte
	tracer        trace.Tracer
}

func build(secret []byte) http.Handler {
	return &service{
		webhookSecret: secret,
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
		fmt.Println(err.Error())
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

	_, ok := event.(*github.PushEvent)
	if !ok {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("skipped\n")) // nolint:errcheck
		return
	}

	// TODO: do something useful...

	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("processed\n")) // nolint:errcheck
}
