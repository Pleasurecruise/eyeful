CREATE SEQUENCE review_lease_token_seq;

CREATE TABLE reviews (
    id               text PRIMARY KEY,
    source           jsonb NOT NULL,
    note             text NOT NULL DEFAULT '',
    status           text NOT NULL,
    result           text NOT NULL DEFAULT '',
    reason           text NOT NULL DEFAULT '',
    attempts         integer NOT NULL DEFAULT 0,
    idempotency_key  text UNIQUE,
    request_hash     bytea NOT NULL,
    lease_owner      text NOT NULL DEFAULT '',
    lease_token      bigint NOT NULL DEFAULT 0,
    lease_until      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz,
    finished_at      timestamptz
);
CREATE INDEX reviews_queue_idx ON reviews (created_at) WHERE status = 'queued';
CREATE INDEX reviews_lease_idx ON reviews (lease_until) WHERE status = 'running';
