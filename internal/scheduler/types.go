package scheduler

import (
	"context"
	"errors"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

type Queue interface {
	Claim(ctx context.Context, owner string, ttl time.Duration) (review.Review, review.Lease, error)
	Renew(ctx context.Context, l review.Lease, ttl time.Duration) error
	Finish(ctx context.Context, l review.Lease, o review.Outcome) error
	Release(ctx context.Context, l review.Lease) error
	Reap(ctx context.Context) (int, error)
}

type Executor interface {
	Execute(ctx context.Context, r review.Review) error
}

type ExecutorFunc func(ctx context.Context, r review.Review) error

type Options struct {
	Owner    string
	Workers  int
	LeaseTTL time.Duration
	Poll     time.Duration
}

var errFenced = errors.New("lease lost")
