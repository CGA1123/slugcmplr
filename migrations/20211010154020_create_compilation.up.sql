CREATE TABLE IF NOT EXISTS compilations (
  event_id VARCHAR(255) NOT NULL,
  target VARCHAR(255) NOT NULL,
  commit_sha varchar(40) NOT NULL,
  tree_sha varchar(40) NOT NULL,
  owner varchar(255) NOT NULL,
  repository varchar(255) NOT NULL,
  created_at TIMESTAMP NOT NULL,

  PRIMARY KEY (event_id, target)
);

CREATE TABLE IF NOT EXISTS compilation_tokens (
  event_id VARCHAR(255) NOT NULL,
  target VARCHAR(255) NOT NULL,
  token VARCHAR(255) NOT NULL,
  created_at TIMESTAMP NOT NULL,
  expired_at TIMESTAMP,

  PRIMARY KEY (token)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_compilation_tokens_uniq_target
ON compilation_tokens (event_id, target)
WHERE expired_at IS NULL;
