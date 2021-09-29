package store

import "context"

// Memory implements an in-memory Querier.
type Memory struct {
	HealthErr error
}

// Health returns nil by default, configurable via HealthErr.
func (m *Memory) Health(_ context.Context) error {
	return m.HealthErr
}
