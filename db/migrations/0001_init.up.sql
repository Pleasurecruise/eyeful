CREATE TABLE users (
    id             text PRIMARY KEY,
    email          text NOT NULL,
    name           text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE accounts (
    id                  text PRIMARY KEY,
    user_id             text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider            text NOT NULL,
    provider_account_id text NOT NULL,
    scopes              text[] NOT NULL DEFAULT '{}',
    access_token        bytea NOT NULL,
    access_expires_at   timestamptz,
    refresh_token       bytea,
    refresh_expires_at  timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_account_id),
    UNIQUE (user_id, provider)
);

CREATE TABLE sessions (
    id         text PRIMARY KEY,
    user_id    text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    ip         text NOT NULL DEFAULT '',
    user_agent text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE api_keys (
    id           text PRIMARY KEY,
    user_id      text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         text NOT NULL,
    prefix       text NOT NULL,
    key_hash     bytea NOT NULL UNIQUE,
    last_used_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX api_keys_user_id_idx ON api_keys (user_id);

CREATE SEQUENCE review_lease_token_seq;

CREATE TABLE reviews (
    id               text PRIMARY KEY,
    user_id          text NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    source           jsonb NOT NULL,
    note             text NOT NULL DEFAULT '',
    status           text NOT NULL,
    result           text NOT NULL DEFAULT '',
    reason           text NOT NULL DEFAULT '',
    attempts         integer NOT NULL DEFAULT 0,
    idempotency_key  text,
    request_hash     bytea NOT NULL,
    lease_owner      text NOT NULL DEFAULT '',
    lease_token      bigint NOT NULL DEFAULT 0,
    lease_until      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz,
    finished_at      timestamptz,
    UNIQUE (user_id, idempotency_key)
);
CREATE INDEX reviews_queue_idx ON reviews (created_at) WHERE status = 'queued';
CREATE INDEX reviews_lease_idx ON reviews (lease_until) WHERE status = 'running';
CREATE INDEX reviews_user_created_idx ON reviews (user_id, created_at DESC);
