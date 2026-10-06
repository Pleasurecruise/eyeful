package review

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pleasurecruise/eyeful/internal/db/sqlc"
)

type PostgresStore struct {
	q           *sqlc.Queries
	maxAttempts int32
}

func NewPostgresStore(pool *pgxpool.Pool, maxAttempts int32) *PostgresStore {
	return &PostgresStore{q: sqlc.New(pool), maxAttempts: max(maxAttempts, 1)}
}

func (s *PostgresStore) Create(ctx context.Context, req Request) (Review, bool, error) {
	if err := req.Validate(); err != nil {
		return Review{}, false, err
	}
	source, err := json.Marshal(req.Source)
	if err != nil {
		return Review{}, false, fmt.Errorf("encode source: %w", err)
	}
	hash := sha256.Sum256(fmt.Appendf(nil, "%s\x00%s", source, req.Note))
	var key *string
	if req.IdempotencyKey != "" {
		key = &req.IdempotencyKey
	}
	row, err := s.q.InsertReview(ctx, sqlc.InsertReviewParams{
		ID:             NewID(),
		UserID:         req.UserID,
		Source:         source,
		Note:           req.Note,
		IdempotencyKey: key,
		RequestHash:    hash[:],
	})
	if err == nil {
		r, err := fromRow(row)
		return r, true, err
	}
	if !errors.Is(err, pgx.ErrNoRows) || key == nil {
		return Review{}, false, fmt.Errorf("insert review: %w", err)
	}
	prev, err := s.q.GetReviewByIdempotencyKey(ctx, sqlc.GetReviewByIdempotencyKeyParams{UserID: req.UserID, IdempotencyKey: key})
	if err != nil {
		return Review{}, false, fmt.Errorf("get review by idempotency key: %w", err)
	}
	if !bytes.Equal(prev.RequestHash, hash[:]) {
		return Review{}, false, ErrIdempotencyConflict
	}
	r, err := fromRow(prev)
	return r, false, err
}

func (s *PostgresStore) Get(ctx context.Context, userID, id string) (Review, error) {
	row, err := s.q.GetReview(ctx, sqlc.GetReviewParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, ErrNotFound
	}
	if err != nil {
		return Review{}, fmt.Errorf("get review: %w", err)
	}
	return fromRow(row)
}

func (s *PostgresStore) List(ctx context.Context, userID string, limit int32) ([]Review, error) {
	rows, err := s.q.ListReviews(ctx, sqlc.ListReviewsParams{UserID: userID, Limit: limit})
	if err != nil {
		return nil, fmt.Errorf("list reviews: %w", err)
	}
	out := make([]Review, 0, len(rows))
	for _, row := range rows {
		r, err := fromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *PostgresStore) Claim(ctx context.Context, owner string, ttl time.Duration) (Review, Lease, error) {
	row, err := s.q.ClaimReview(ctx, sqlc.ClaimReviewParams{Owner: owner, TtlSeconds: ttl.Seconds()})
	if errors.Is(err, pgx.ErrNoRows) {
		return Review{}, Lease{}, ErrQueueEmpty
	}
	if err != nil {
		return Review{}, Lease{}, fmt.Errorf("claim review: %w", err)
	}
	r, err := fromRow(row)
	return r, Lease{ReviewID: row.ID, Owner: owner, FencingToken: row.LeaseToken}, err
}

func (s *PostgresStore) Renew(ctx context.Context, l Lease, ttl time.Duration) error {
	n, err := s.q.RenewLease(ctx, sqlc.RenewLeaseParams{ID: l.ReviewID, LeaseToken: l.FencingToken, TtlSeconds: ttl.Seconds()})
	if err != nil {
		return fmt.Errorf("renew lease: %w", err)
	}
	if n == 0 {
		return ErrFenced
	}
	return nil
}

func (s *PostgresStore) Finish(ctx context.Context, l Lease, o Outcome) error {
	if err := o.Validate(); err != nil {
		return err
	}
	n, err := s.q.FinishReview(ctx, sqlc.FinishReviewParams{Result: string(o.Result), Reason: o.Reason, ID: l.ReviewID, LeaseToken: l.FencingToken})
	if err != nil {
		return fmt.Errorf("finish review: %w", err)
	}
	if n == 0 {
		return ErrFenced
	}
	return nil
}

func (s *PostgresStore) Release(ctx context.Context, l Lease) error {
	n, err := s.q.ReleaseReview(ctx, sqlc.ReleaseReviewParams{ID: l.ReviewID, LeaseToken: l.FencingToken})
	if err != nil {
		return fmt.Errorf("release review: %w", err)
	}
	if n == 0 {
		return ErrFenced
	}
	return nil
}

func (s *PostgresStore) Reap(ctx context.Context) (int, error) {
	n, err := s.q.ReapExpired(ctx, s.maxAttempts)
	if err != nil {
		return 0, fmt.Errorf("reap reviews: %w", err)
	}
	return int(n), nil
}

func fromRow(row sqlc.Review) (Review, error) {
	var src Source
	if err := json.Unmarshal(row.Source, &src); err != nil {
		return Review{}, fmt.Errorf("decode review %s source: %w", row.ID, err)
	}
	status, err := ParseStatus(row.Status)
	if err != nil {
		return Review{}, fmt.Errorf("review %s: %w", row.ID, err)
	}
	result, err := ParseResult(row.Result)
	if err != nil {
		return Review{}, fmt.Errorf("review %s: %w", row.ID, err)
	}
	return Review{
		ID:         row.ID,
		UserID:     row.UserID,
		Source:     src,
		Note:       row.Note,
		Status:     status,
		Result:     result,
		Reason:     row.Reason,
		Attempts:   row.Attempts,
		CreatedAt:  timeOf(row.CreatedAt),
		StartedAt:  timeOf(row.StartedAt),
		FinishedAt: timeOf(row.FinishedAt),
	}, nil
}

func timeOf(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func (s *PostgresStore) Delete(ctx context.Context, userID, id string) error {
	n, err := s.q.DeleteReview(ctx, sqlc.DeleteReviewParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete review: %w", err)
	}
	if n > 0 {
		return nil
	}
	if _, err := s.Get(ctx, userID, id); err != nil {
		return err
	}
	return ErrRunning
}
