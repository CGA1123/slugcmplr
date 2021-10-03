CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS queued_jobs (
  id UUID NOT NULL,
  queue_name VARCHAR(255) NOT NULL,
  queued_at TIMESTAMP NOT NULL,
  scheduled_at TIMESTAMP NOT NULL,
  data BYTEA NOT NULL,
  attempt int NOT NULL,

  PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS dead_jobs (
  id UUID NOT NULL,
  queue_name VARCHAR(255) NOT NULL,
  queued_at TIMESTAMP NOT NULL,
  scheduled_at TIMESTAMP NOT NULL,
  data BYTEA NOT NULL,
  attempt int NOT NULL,

  PRIMARY KEY (id)
);
