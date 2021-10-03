package obs

import (
	"context"
	"strings"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// DBTX describes the database.
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

var _ DBTX = (*DB)(nil)

// DB implements the DBTX interface with opentelemetry instrumentation.
type DB struct {
	db DBTX
	t  trace.Tracer
}

// NewDB builds a new DBTX with opentelemetry tracing.
func NewDB(db DBTX) *DB {
	return &DB{
		db: db,
		t:  otel.Tracer("github.com/CGA1123/slugcmplr/store"),
	}
}

// WithTx builds a new DB using the given transaction connection.
func (o *DB) WithTx(tx pgx.Tx) *DB {
	return &DB{
		db: tx,
		t:  o.t,
	}
}

// Exec instruments the underlying call to Exec.
func (o *DB) Exec(ctx context.Context, q string, args ...interface{}) (pgconn.CommandTag, error) {
	cctx, span := o.buildSpan(ctx, "exec", q)
	defer span.End()

	tag, err := o.db.Exec(cctx, q, args...)
	queryErr(span, err)

	return tag, err
}

// Query instruments the underlying call to Query.
func (o *DB) Query(ctx context.Context, q string, args ...interface{}) (pgx.Rows, error) {
	cctx, span := o.buildSpan(ctx, "query", q)
	defer span.End()

	rows, err := o.db.Query(cctx, q, args...)
	queryErr(span, err)

	return rows, err
}

// QueryRow instruments the underlying call to QueryRow.
func (o *DB) QueryRow(ctx context.Context, q string, args ...interface{}) pgx.Row {
	cctx, span := o.buildSpan(ctx, "query_row", q)
	defer span.End()

	return o.db.QueryRow(cctx, q, args...)
}

func (o *DB) buildSpan(ctx context.Context, operation, query string) (context.Context, trace.Span) {
	name := queryName(query)
	cctx, span := o.t.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBStatementKey.String(name),
			semconv.DBOperationKey.String(operation),
			attribute.String("type", "db"),
		),
	)

	return cctx, span
}

func queryErr(span trace.Span, err error) {
	if err != nil {
		if pgerr, ok := err.(*pgconn.PgError); ok {
			span.SetAttributes(
				attribute.String("sql.error_code", pgerr.Code),
				attribute.String("sql.error_severity", pgerr.Severity),
				attribute.String("sql.error_message", pgerr.Message),
			)
		}
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
