package store

import (
	"context"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var _ DBTX = (*ObsQueries)(nil)

// ObsQueries implements the DBTX interface with opentelemetry instrumentation.
type ObsQueries struct {
	db DBTX
	t  trace.Tracer
}

// NewObsQueries builds a new DBTX with opentelemetry tracing.
func NewObsQueries(db DBTX) *ObsQueries {
	return &ObsQueries{
		db: db,
		t:  otel.Tracer("github.com/CGA1123/slugcmplr/store"),
	}
}

// WithTx builds a new ObsQueries using the given transaction connection.
func (o *ObsQueries) WithTx(tx pgx.Tx) *ObsQueries {
	return &ObsQueries{
		db: tx,
		t:  o.t,
	}
}

// Exec instruments the underlying call to Exec.
func (o *ObsQueries) Exec(ctx context.Context, q string, args ...interface{}) (pgconn.CommandTag, error) {
	cctx, span := o.buildSpan(ctx, "exec", q)
	defer span.End()

	tag, err := o.db.Exec(cctx, q, args...)
	queryErr(span, err)

	return tag, err
}

// Query instruments the underlying call to Query.
func (o *ObsQueries) Query(ctx context.Context, q string, args ...interface{}) (pgx.Rows, error) {
	cctx, span := o.buildSpan(ctx, "query", q)
	defer span.End()

	rows, err := o.db.Query(cctx, q, args...)
	queryErr(span, err)

	return rows, err
}

// QueryRow instruments the underlying call to QueryRow.
func (o *ObsQueries) QueryRow(ctx context.Context, q string, args ...interface{}) pgx.Row {
	cctx, span := o.buildSpan(ctx, "query_row", q)
	defer span.End()

	return o.db.QueryRow(cctx, q, args...)
}

func (o *ObsQueries) buildSpan(ctx context.Context, operation, query string) (context.Context, trace.Span) {
	name := queryName(query)
	cctx, span := o.t.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("type", "sql"),
			attribute.String("sql.name", name),
			attribute.String("sql.operation", operation),
		),
	)

	return cctx, span
}

func queryErr(span trace.Span, err error) {
	if err != nil {
		span.SetAttributes(
			attribute.String("sql.resolution", "failed"),
		)

		if pgerr, ok := err.(*pgconn.PgError); ok {
			span.SetAttributes(
				attribute.String("sql.error_code", pgerr.Code),
				attribute.String("sql.error_severity", pgerr.Severity),
				attribute.String("sql.error_message", pgerr.Message),
			)
		}
	} else {
		span.SetAttributes(
			attribute.String("sql.resolution", "succeeded"),
		)
	}
}

func queryName(query string) string {
	if !strings.HasPrefix(query, "-- name: ") {
		return "unknown"
	}

	name := strings.SplitN(strings.TrimPrefix(query, "-- name: "), " ", 2)
	if len(name) == 0 {
		return "unknown"
	}

	return name[0]
}
