package store

import (
	"context"
	"fmt"

	"github.com/cga1123/slugcmplr/obs"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Build creates a new *Queries using the connection string provided.
// TODO: should this take a connection pool rather than a connstring? Enable
// sharing pools across multiple store interfaces?
func Build(connstr string) (*Queries, error) {
	config, err := pgxpool.ParseConfig(connstr)
	if err != nil {
		return nil, fmt.Errorf("error parsing connstr: %w", err)
	}

	config.MaxConns = 5
	config.MinConns = 5

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("error creating db connection pool: %w", err)
	}

	return New(obs.NewDB(pool)), nil
}
