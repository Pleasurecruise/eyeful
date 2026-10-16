package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/db"
)

const reviewCols = `id, source, status, result, reason, attempts, idempotency_key, request_hash, lease_owner, lease_token, lease_until, created_at, started_at, finished_at`

type Store struct {
	q           db.Queryer
	maxAttempts int32
}

func New(q db.Queryer, maxAttempts int32) *Store {
	return &Store{q: q, maxAttempts: max(maxAttempts, 1)}
}

func (s *Store) Create(ctx context.Context, req Request) (Review, bool, error) {
	if err := req.Source.Validate(); err != nil {
		return Review{}, false, err
	}
	source, err := json.Marshal(req.Source)
	if err != nil {
		return Review{}, false, fmt.Errorf("encode source: %w", err)
	}
	hash := sha256.Sum256(source)
	row, ok, err := one(ctx, s.q, `
		INSERT INTO reviews (id, source, status, idempotency_key, request_hash, created_at)
		VALUES (?, ?, 'queued', NULLIF(?, ''), ?, CAST(? AS INTEGER))
		ON CONFLICT (idempotency_key) DO NOTHING
		RETURNING `+reviewCols, NewID(), string(source), req.IdempotencyKey, hex.EncodeToString(hash[:]), millis(time.Now()))
	if err != nil {
		return Review{}, false, fmt.Errorf("insert review: %w", err)
	}
	if ok {
		r, err := fromRow(row)
		return r, true, err
	}
	if req.IdempotencyKey == "" {
		return Review{}, false, errors.New("insert review: no row")
	}
	prev, ok, err := one(ctx, s.q, `SELECT `+reviewCols+` FROM reviews WHERE idempotency_key = ?`, req.IdempotencyKey)
	if err != nil {
		return Review{}, false, fmt.Errorf("get review by idempotency key: %w", err)
	}
	if !ok {
		return Review{}, false, fmt.Errorf("get review by idempotency key: %w", ErrNotFound)
	}
	got, _ := prev.Text("request_hash")
	if got != hex.EncodeToString(hash[:]) {
		return Review{}, false, ErrIdempotencyConflict
	}
	r, err := fromRow(prev)
	return r, false, err
}

func (s *Store) Get(ctx context.Context, id string) (Review, error) {
	row, ok, err := one(ctx, s.q, `SELECT `+reviewCols+` FROM reviews WHERE id = ?`, id)
	if err != nil {
		return Review{}, fmt.Errorf("get review: %w", err)
	}
	if !ok {
		return Review{}, ErrNotFound
	}
	return fromRow(row)
}

func (s *Store) Claim(ctx context.Context, owner string, ttl time.Duration) (Review, Lease, error) {
	now := millis(time.Now())
	row, ok, err := one(ctx, s.q, `
		UPDATE reviews SET
			status = 'running',
			lease_owner = ?,
			lease_token = (SELECT COALESCE(MAX(lease_token), 0) + 1 FROM reviews),
			lease_until = CAST(? AS INTEGER) + CAST(? AS INTEGER),
			attempts = attempts + 1,
			started_at = COALESCE(started_at, CAST(? AS INTEGER))
		WHERE id = (
			SELECT id FROM reviews
			WHERE status = 'queued'
			ORDER BY created_at, rowid
			LIMIT 1
		)
		RETURNING `+reviewCols, owner, now, strconv.FormatInt(ttl.Milliseconds(), 10), now)
	if err != nil {
		return Review{}, Lease{}, fmt.Errorf("claim review: %w", err)
	}
	if !ok {
		return Review{}, Lease{}, ErrQueueEmpty
	}
	token, ok, err := row.Int("lease_token")
	if err != nil {
		return Review{}, Lease{}, fmt.Errorf("claim review: %w", err)
	}
	if !ok {
		return Review{}, Lease{}, errors.New("claim review: lease token")
	}
	r, err := fromRow(row)
	return r, Lease{ReviewID: r.ID, Owner: owner, FencingToken: token}, err
}

func (s *Store) Renew(ctx context.Context, l Lease, ttl time.Duration) error {
	return s.touch(ctx, "renew lease", `
		UPDATE reviews SET lease_until = CAST(? AS INTEGER) + CAST(? AS INTEGER)
		WHERE id = ? AND lease_token = CAST(? AS INTEGER) AND status = 'running'
		RETURNING id`, millis(time.Now()), strconv.FormatInt(ttl.Milliseconds(), 10), l.ReviewID, strconv.FormatInt(l.FencingToken, 10))
}

func (s *Store) Finish(ctx context.Context, l Lease, o Outcome) error {
	if err := o.Validate(); err != nil {
		return err
	}
	return s.touch(ctx, "finish review", `
		UPDATE reviews SET status = 'done', result = ?, reason = ?, finished_at = CAST(? AS INTEGER), lease_owner = ''
		WHERE id = ? AND lease_token = CAST(? AS INTEGER) AND status = 'running'
		RETURNING id`, string(o.Result), o.Reason, millis(time.Now()), l.ReviewID, strconv.FormatInt(l.FencingToken, 10))
}

func (s *Store) Release(ctx context.Context, l Lease) error {
	return s.touch(ctx, "release review", `
		UPDATE reviews SET status = 'queued', attempts = attempts - 1, lease_owner = ''
		WHERE id = ? AND lease_token = CAST(? AS INTEGER) AND status = 'running'
		RETURNING id`, l.ReviewID, strconv.FormatInt(l.FencingToken, 10))
}

func (s *Store) Reap(ctx context.Context) (int, error) {
	maxAttempts := strconv.FormatInt(int64(s.maxAttempts), 10)
	now := millis(time.Now())
	rows, err := s.q.Query(ctx, `
		UPDATE reviews SET
			status = CASE WHEN attempts >= CAST(? AS INTEGER) THEN 'done' ELSE 'queued' END,
			result = CASE WHEN attempts >= CAST(? AS INTEGER) THEN 'none' ELSE result END,
			reason = CASE WHEN attempts >= CAST(? AS INTEGER) THEN 'worker lost ' || attempts || ' times' ELSE reason END,
			finished_at = CASE WHEN attempts >= CAST(? AS INTEGER) THEN CAST(? AS INTEGER) ELSE finished_at END,
			lease_owner = ''
		WHERE status = 'running' AND lease_until IS NOT NULL AND lease_until < CAST(? AS INTEGER)
		RETURNING id`, maxAttempts, maxAttempts, maxAttempts, maxAttempts, now, now)
	if err != nil {
		return 0, fmt.Errorf("reap reviews: %w", err)
	}
	return len(rows), nil
}

func (s *Store) touch(ctx context.Context, op, query string, params ...string) error {
	_, ok, err := one(ctx, s.q, query, params...)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if !ok {
		return ErrFenced
	}
	return nil
}

func one(ctx context.Context, q db.Queryer, query string, params ...string) (db.Row, bool, error) {
	rows, err := q.Query(ctx, query, params...)
	if err != nil {
		return db.Row{}, false, err
	}
	if len(rows) == 0 {
		return db.Row{}, false, nil
	}
	return rows[0], true, nil
}

func fromRow(row db.Row) (Review, error) {
	id, ok := row.Text("id")
	if !ok {
		return Review{}, errors.New("review row has no id")
	}
	raw, ok := row.Text("source")
	if !ok {
		return Review{}, fmt.Errorf("review %s has no source", id)
	}
	var src Source
	if err := json.Unmarshal([]byte(raw), &src); err != nil {
		return Review{}, fmt.Errorf("decode review %s source: %w", id, err)
	}
	statusText, _ := row.Text("status")
	status, err := ParseStatus(statusText)
	if err != nil {
		return Review{}, fmt.Errorf("review %s: %w", id, err)
	}
	resultText, _ := row.Text("result")
	result, err := ParseResult(resultText)
	if err != nil {
		return Review{}, fmt.Errorf("review %s: %w", id, err)
	}
	attempts, err := row.Int32("attempts")
	if err != nil {
		return Review{}, fmt.Errorf("review %s attempts: %w", id, err)
	}
	reason, _ := row.Text("reason")
	created, err := row.Time("created_at")
	if err != nil {
		return Review{}, fmt.Errorf("review %s created_at: %w", id, err)
	}
	started, err := row.Time("started_at")
	if err != nil {
		return Review{}, fmt.Errorf("review %s started_at: %w", id, err)
	}
	finished, err := row.Time("finished_at")
	if err != nil {
		return Review{}, fmt.Errorf("review %s finished_at: %w", id, err)
	}
	return Review{
		ID: id, Source: src, Status: status, Result: result, Reason: reason, Attempts: attempts,
		CreatedAt: created, StartedAt: started, FinishedAt: finished,
	}, nil
}

func millis(t time.Time) string {
	return strconv.FormatInt(t.UnixMilli(), 10)
}
