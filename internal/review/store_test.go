package review_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/db/dbtest"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/review/reviewtest"
)

type store interface {
	Create(ctx context.Context, req review.Request) (review.Review, bool, error)
	Get(ctx context.Context, id string) (review.Review, error)
	Claim(ctx context.Context, owner string, ttl time.Duration) (review.Review, review.Lease, error)
	Renew(ctx context.Context, l review.Lease, ttl time.Duration) error
	Finish(ctx context.Context, l review.Lease, o review.Outcome) error
	Release(ctx context.Context, l review.Lease) error
	Reap(ctx context.Context) (int, error)
}

var ctx = context.Background()

func forEachStore(t *testing.T, maxAttempts int32, test func(t *testing.T, s store)) {
	t.Run("memory", func(t *testing.T) { test(t, reviewtest.NewMemoryStore(maxAttempts)) })
	t.Run("postgres", func(t *testing.T) {
		test(t, review.NewPostgresStore(dbtest.Open(t), maxAttempts))
	})
}

func pr() review.Request {
	return review.Request{Source: review.Source{Repo: "o/r", Base: "main", Head: "feature"}}
}

func TestIdempotency(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		req := pr()
		req.IdempotencyKey = "k"
		first, created, err := s.Create(ctx, req)
		if err != nil || !created {
			t.Fatalf("first create: %v %v", created, err)
		}
		again, created, err := s.Create(ctx, req)
		if err != nil || created || again.ID != first.ID {
			t.Fatalf("replay: id %s created %v err %v", again.ID, created, err)
		}
		if _, created, err := s.Create(ctx, pr()); err != nil || !created {
			t.Fatalf("no key: %v %v", created, err)
		}
		req.Note = "different"
		if _, _, err := s.Create(ctx, req); !errors.Is(err, review.ErrIdempotencyConflict) {
			t.Fatalf("conflict: got %v", err)
		}
	})
}

func TestFencing(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr())
		_, old, err := s.Claim(ctx, "a", 10*time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
		if n, err := s.Reap(ctx); err != nil || n != 1 {
			t.Fatalf("reaped %d (%v), want 1", n, err)
		}
		_, cur, err := s.Claim(ctx, "b", time.Minute)
		if err != nil || cur.FencingToken <= old.FencingToken {
			t.Fatalf("reclaim: %+v %v", cur, err)
		}
		if err := s.Finish(ctx, old, review.Outcome{Result: review.ResultComplete}); !errors.Is(err, review.ErrFenced) {
			t.Fatalf("stale finish: got %v", err)
		}
		if err := s.Renew(ctx, old, time.Minute); !errors.Is(err, review.ErrFenced) {
			t.Fatalf("stale renew: got %v", err)
		}
		if err := s.Finish(ctx, cur, review.Outcome{Result: review.ResultComplete}); err != nil {
			t.Fatal(err)
		}
		got, _ := s.Get(ctx, r.ID)
		if got.Status != review.StatusDone || got.Result != review.ResultComplete || got.Attempts != 2 {
			t.Fatalf("final: %+v", got)
		}
	})
}

func TestReap(t *testing.T) {
	forEachStore(t, 1, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr())
		if _, _, err := s.Claim(ctx, "a", 10*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
		if _, err := s.Reap(ctx); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Get(ctx, r.ID); got.Status != review.StatusDone || got.Result != review.ResultNone || got.Reason == "" {
			t.Fatalf("got %+v, want done with no result and a reason", got)
		}
	})
}

func TestFinish(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr())
		_, l, _ := s.Claim(ctx, "a", time.Minute)
		if err := s.Finish(ctx, l, review.Outcome{Result: review.ResultNone}); err == nil {
			t.Fatal("finish with no result and no reason must fail")
		}
		if err := s.Finish(ctx, l, review.Outcome{Result: review.ResultNone, Reason: "clone failed"}); err != nil {
			t.Fatal(err)
		}
		got, _ := s.Get(ctx, r.ID)
		if got.Status != review.StatusDone || got.Result != review.ResultNone || got.Reason != "clone failed" || got.FinishedAt.IsZero() {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestRelease(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr())
		_, l, _ := s.Claim(ctx, "a", time.Minute)
		if err := s.Release(ctx, l); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Get(ctx, r.ID); got.Status != review.StatusQueued || got.Attempts != 0 {
			t.Fatalf("got %+v", got)
		}
	})
}
