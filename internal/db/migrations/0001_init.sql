CREATE TABLE reviews (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    status TEXT NOT NULL,
    result TEXT NOT NULL DEFAULT '',
    reason TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    idempotency_key TEXT UNIQUE,
    request_hash TEXT NOT NULL,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_token INTEGER NOT NULL DEFAULT 0,
    lease_until INTEGER,
    created_at INTEGER NOT NULL,
    started_at INTEGER,
    finished_at INTEGER
) STRICT;

CREATE INDEX reviews_queue_idx ON reviews (created_at) WHERE status = 'queued';

CREATE INDEX reviews_lease_idx ON reviews (lease_until) WHERE status = 'running';
