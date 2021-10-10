-- name: Create :exec
INSERT INTO compilations (
  event_id,
  target,
  commit_sha,
  tree_sha,
  owner,
  repository,
  created_at,
  updated_at
) VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  NOW(),
  NOW()
);


-- name: CreateToken :exec
INSERT INTO compilation_tokens (
  event_id,
  target,
  token,
  created_at
) VALUES (
  $1,
  $2,
  $3,
  NOW()
);

-- name: ExpireToken :exec
UPDATE compilation_tokens
SET expired_at = NOW()
WHERE token = $1;

-- name: FetchWithToken :one
SELECT C.*
FROM compilations C
JOIN compilation_tokens CT
ON CT.event_id = C.event_id
AND CT.target = C.target
WHERE CT.token = $1
AND CT.expired_at IS NULL
LIMIT 1;
