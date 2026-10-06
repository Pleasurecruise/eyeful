-- name: InsertReview :one
INSERT INTO reviews (id, user_id, source, note, status, idempotency_key, request_hash)
VALUES ($1, $2, $3, $4, 'queued', $5, $6)
ON CONFLICT (user_id, idempotency_key) DO NOTHING
RETURNING *;

-- name: GetReviewByIdempotencyKey :one
SELECT * FROM reviews WHERE user_id = $1 AND idempotency_key = $2;

-- name: GetReview :one
SELECT * FROM reviews WHERE id = $1 AND user_id = $2;

-- name: ListReviews :many
SELECT * FROM reviews WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2;

-- name: ClaimReview :one
UPDATE reviews SET
    status = 'running',
    lease_owner = @owner,
    lease_token = nextval('review_lease_token_seq'),
    lease_until = now() + make_interval(secs => @ttl_seconds::float8),
    attempts = attempts + 1,
    started_at = coalesce(started_at, now())
WHERE id = (
    SELECT id FROM reviews WHERE status = 'queued'
    ORDER BY created_at LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: RenewLease :execrows
UPDATE reviews SET lease_until = now() + make_interval(secs => @ttl_seconds::float8)
WHERE id = @id AND lease_token = @lease_token AND status = 'running';

-- name: FinishReview :execrows
UPDATE reviews SET status = 'done', result = @result, reason = @reason, finished_at = now(), lease_owner = ''
WHERE id = @id AND lease_token = @lease_token AND status = 'running';

-- name: ReleaseReview :execrows
UPDATE reviews SET status = 'queued', attempts = attempts - 1, lease_owner = ''
WHERE id = @id AND lease_token = @lease_token AND status = 'running';

-- name: ReapExpired :execrows
UPDATE reviews SET
    status = CASE WHEN attempts >= @max_attempts::int THEN 'done' ELSE 'queued' END,
    result = CASE WHEN attempts >= @max_attempts::int THEN 'none' ELSE result END,
    reason = CASE WHEN attempts >= @max_attempts::int THEN 'worker lost ' || attempts || ' times' ELSE reason END,
    finished_at = CASE WHEN attempts >= @max_attempts::int THEN now() ELSE finished_at END,
    lease_owner = ''
WHERE status = 'running' AND lease_until < now();

-- name: DeleteReview :execrows
DELETE FROM reviews
WHERE id = $1 AND user_id = $2 AND status IN ('queued', 'done');
