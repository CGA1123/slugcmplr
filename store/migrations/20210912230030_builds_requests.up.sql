CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS build_requests (
  id uuid NOT NULL,
  targets varchar(255) ARRAY NOT NULL,
  created_at TIMESTAMP NOT NULL,
  updated_at TIMESTAMP NOT NULL,
  commit_sha varchar(40) NOT NULL,
  tree_sha varchar(40) NOT NULL,
  owner varchar(255) NOT NULL,
  repository varchar(255) NOT NULL,

  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS receive_tokens (
  id uuid NOT NULL,
  build_request_id uuid NOT NULL,
  build_request_target varchar(255) NOT NULL,
  created_at TIMESTAMP NOT NULL,
  claimed_at TIMESTAMP,
  token varchar(255) NOT NULL,

  PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_uniq_unclaimed_receive_tokens
ON receive_tokens (build_request_id, build_request_target)
WHERE claimed_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_uniq_token_receive_tokens
ON receive_tokens (token);

CREATE TABLE IF NOT EXISTS build_tokens (
  id uuid NOT NULL,
  build_request_id uuid NOT NULL,
  build_request_target varchar(255) NOT NULL,
  created_at TIMESTAMP NOT NULL,
  expired_at TIMESTAMP,
  token varchar(255) NOT NULL,

  PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_uniq_active_build_tokens
ON build_tokens (build_request_id, build_request_target)
WHERE expired_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_uniq_token_build_tokens
ON build_tokens (token);
