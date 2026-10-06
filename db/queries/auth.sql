-- name: CreateUser :one
INSERT INTO users (id, email, name) VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE lower(email) = lower($1);

-- name: GetAccountUser :one
SELECT sqlc.embed(u)
FROM accounts a JOIN users u ON u.id = a.user_id
WHERE a.provider = $1 AND a.provider_account_id = $2;

-- name: CreateAccount :exec
INSERT INTO accounts (id, user_id, provider, provider_account_id, scopes,
                      access_token, access_expires_at, refresh_token, refresh_expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: UpdateAccountGrant :exec
UPDATE accounts
SET scopes = $3, access_token = $4, access_expires_at = $5, refresh_token = $6,
    refresh_expires_at = $7, updated_at = now()
WHERE provider = $1 AND provider_account_id = $2;

-- name: GetAccountGrant :one
SELECT provider_account_id, scopes, access_token, access_expires_at, refresh_token, refresh_expires_at
FROM accounts WHERE user_id = $1 AND provider = $2
FOR UPDATE;

-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, expires_at, ip, user_agent)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetSessionUser :one
SELECT sqlc.embed(u), s.id AS session_id
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now();

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= now();

-- name: CreateAPIKey :one
INSERT INTO api_keys (id, user_id, name, prefix, key_hash)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListAPIKeys :many
SELECT * FROM api_keys WHERE user_id = $1 ORDER BY created_at DESC;

-- name: DeleteAPIKey :execrows
DELETE FROM api_keys WHERE id = $1 AND user_id = $2;

-- name: UseAPIKey :one
UPDATE api_keys k SET last_used_at = now()
WHERE k.key_hash = $1
RETURNING k.id, k.user_id;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: UpdateUser :one
UPDATE users SET name = $2, updated_at = now() WHERE id = $1
RETURNING *;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

-- name: ListSessions :many
SELECT * FROM sessions WHERE user_id = $1 AND expires_at > now() ORDER BY created_at DESC;

-- name: DeleteSessionByID :execrows
DELETE FROM sessions WHERE id = $1 AND user_id = $2;

-- name: GetAPIKey :one
SELECT * FROM api_keys WHERE id = $1 AND user_id = $2;

-- name: UpdateAPIKey :one
UPDATE api_keys SET name = $3 WHERE id = $1 AND user_id = $2
RETURNING *;
