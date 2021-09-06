-- name: CreateBuildRequest :one
INSERT INTO build_requests (
  id,
  targets,
  commit_sha,
  tree_sha,
  owner,
  repository,
  created_at,
  updated_at
)
VALUES (
  $1,
  $2,
  $3,
  $4,
  $5,
  $6,
  NOW(),
  NOW()
)
ON CONFLICT (id) DO UPDATE
SET updated_at = NOW()
RETURNING id;

-- name: GetBuildRequest :one
SELECT *
FROM build_requests
WHERE id = $1;

-- name: GetBuildRequestFromReceiveToken :one
SELECT B.*, T.build_request_target AS target
FROM build_requests B
JOIN receive_tokens T ON B.id = T.build_request_id
WHERE T.token = $1
AND T.claimed_at IS NULL;

-- name: GetBuildRequestFromBuildToken :one
SELECT B.*, T.build_request_target AS target
FROM build_requests B
JOIN build_tokens T ON B.id = T.build_request_id
WHERE T.token = $1
AND T.expired_at IS NULL;

-- name: CreateReceiveToken :one
INSERT INTO receive_tokens (
  build_request_id,
  build_request_target,
  token,
  id,
  created_at
)
VALUES (
  $1,
  $2,
  $3,
  uuid_generate_v4(),
  NOW()
)
RETURNING id;

-- name: ClaimReceiveToken :exec
UPDATE receive_tokens
SET claimed_at = NOW()
WHERE claimed_at IS NULL
AND build_request_target = $1
AND build_request_id = $2;

-- name: CreateBuildToken :one
INSERT INTO build_tokens (
  build_request_id,
  build_request_target,
  token,
  id,
  created_at
)
VALUES (
  $1,
  $2,
  $3,
  uuid_generate_v4(),
  NOW()
)
RETURNING id;

-- name: ExpireBuildToken :exec
UPDATE build_tokens
SET expired_at = NOW()
WHERE expired_at IS NULL
AND build_request_id = $1
AND build_request_target = $2;
