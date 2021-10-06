-- name: Dequeue :one
DELETE FROM queued_jobs
USING (
  SELECT *
  FROM queued_jobs
  WHERE queued_jobs.queue_name = $1
  ORDER BY scheduled_at ASC
  LIMIT 1
  FOR UPDATE SKIP LOCKED
) jobs
WHERE jobs.id = queued_jobs.id
RETURNING queued_jobs.*
;

-- name: Enqueue :one
INSERT INTO queued_jobs (
  id,
  queue_name,
  queued_at,
  scheduled_at,
  data,
  attempt
) VALUES (
  uuid_generate_v4(),
  $1,
  NOW(),
  $2,
  $3,
  $4
)
RETURNING id
;

-- name: DeadLetter :exec
INSERT INTO dead_jobs (
  id,
  queue_name,
  queued_at,
  scheduled_at,
  data,
  attempt
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6
);

-- name: DeadLetterCount :one
SELECT COUNT(*) FROM dead_jobs;
