package queue_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cga1123/slugcmplr/queue"
	"github.com/cga1123/slugcmplr/queue/store"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pool(t *testing.T) *pgxpool.Pool {
	url := os.Getenv("SLUGCMPLR_TEST_DB_URL")
	if url == "" {
		url = "postgres://localhost:5432/slugcmplr_test?sslmode=disable"
	}
	config, err := pgxpool.ParseConfig(url)
	require.NoError(t, err, "Connecting to the database should succeed.")

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
	require.NoError(t, err, "Creating a connection pool should succeed")

	return pool
}

func dbtest(t *testing.T) {
	if _, ok := os.LookupEnv("SLUGCMPLR_DB"); ok {
		return
	}

	t.Skipf("set SLUGCMPLR_DB to run DB dependent tests")
}

// nolint:paralleltest
func Test_EnqDeq(t *testing.T) {
	dbtest(t)

	db := pool(t)

	jobs := make([]string, 0, 3)
	q := queue.New(db, "test_queue", queue.NoRetryWorker(func(_ context.Context, j store.QueuedJob) error {
		jobs = append(jobs, string(j.Data))

		return nil
	}))

	ctx := context.Background()

	now := time.Now()

	_, err := q.Enq(ctx, []byte("1"), queue.ScheduledAt(now.Add(-time.Minute)))
	require.NoError(t, err)

	_, err = q.Enq(ctx, []byte("2"), queue.ScheduledAt(now.Add(-time.Second)))
	require.NoError(t, err)

	_, err = q.Enq(ctx, []byte("3"), queue.ScheduledAt(now))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		require.NoError(t, q.Deq(ctx), "Dequeueing should be successful")
	}

	assert.Equal(t, []string{"1", "2", "3"}, jobs, "Should dequeue in order")
	assert.ErrorIs(t, q.Deq(ctx), pgx.ErrNoRows, "Expected dequeueing an empty queue to error")
}

// nolint:paralleltest
func Test_Retries(t *testing.T) {
	dbtest(t)

	db := pool(t)

	jobs := make([]string, 0, 3)
	q := queue.New(db, "test_queue", queue.RetryWorker(
		2,
		queue.ConstantBackoff(time.Duration(0)),
		func(_ context.Context, j store.QueuedJob) error {
			jobs = append(jobs, string(j.Data))

			if j.Attempt < int32(2) {
				return fmt.Errorf("an error")
			}

			return nil
		}))

	ctx := context.Background()

	_, err := q.Enq(ctx, []byte("1"), queue.ScheduledAt(time.Now().Add(-time.Minute)))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		require.NoError(t, q.Deq(ctx), "Dequeueing should be successful")
	}

	assert.Equal(t, []string{"1", "1", "1"}, jobs, "Should dequeue in order")
	assert.ErrorIs(t, q.Deq(ctx), pgx.ErrNoRows, "Expected dequeueing an empty queue to error")
}

// nolint:paralleltest
func Test_DeadLetter(t *testing.T) {
	dbtest(t)

	db := pool(t)
	defer func() {
		_, err := db.Exec(context.Background(), "TRUNCATE dead_jobs")
		require.NoError(t, err, "Truncating should succeed")
	}()

	jobs := make([]string, 0, 3)
	q := queue.New(db, "test_queue", queue.RetryWorker(
		2,
		queue.ConstantBackoff(time.Duration(0)),
		func(_ context.Context, j store.QueuedJob) error {
			jobs = append(jobs, string(j.Data))

			return fmt.Errorf("an error")
		}))

	ctx := context.Background()

	_, err := q.Enq(ctx, []byte("1"), queue.ScheduledAt(time.Now().Add(-time.Minute)))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		require.NoError(t, q.Deq(ctx), "Dequeueing should be successful")
	}

	assert.Equal(t, []string{"1", "1", "1"}, jobs, "Should dequeue in order")
	assert.ErrorIs(t, q.Deq(ctx), pgx.ErrNoRows, "Expected dequeueing an empty queue to error")

	deadLetters, err := store.New(db).DeadLetterCount(ctx)
	require.NoError(t, err, "Fetching count of dead letters should not fail")

	assert.Equal(t, int64(1), deadLetters, "There should be a single dead letter")
}
