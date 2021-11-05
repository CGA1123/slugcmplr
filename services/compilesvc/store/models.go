// Code generated by sqlc. DO NOT EDIT.

package store

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type Compilation struct {
	EventID    string
	Target     string
	CommitSha  string
	TreeSha    string
	Owner      string
	Repository string
	CreatedAt  time.Time
}

type CompilationToken struct {
	EventID   string
	Target    string
	Token     string
	CreatedAt time.Time
	ExpiredAt sql.NullTime
}

type DeadJob struct {
	ID          uuid.UUID
	QueueName   string
	QueuedAt    time.Time
	ScheduledAt time.Time
	Data        []byte
	Attempt     int32
}

type QueuedJob struct {
	ID          uuid.UUID
	QueueName   string
	QueuedAt    time.Time
	ScheduledAt time.Time
	Data        []byte
	Attempt     int32
}