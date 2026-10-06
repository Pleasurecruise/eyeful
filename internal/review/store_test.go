package review_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/db/dbtest"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/review/reviewtest"
)

type store interface {
	Create(ctx context.Context, req review.Request) (review.Review, bool, error)
	Get(ctx context.Context, userID, id string) (review.Review, error)
	List(ctx context.Context, userID string, limit int32) ([]review.Review, error)
	Delete(ctx context.Context, userID, id string) error
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
		pool := dbtest.Open(t)
		dbtest.CreateUser(t, pool, "usr_a")
		dbtest.CreateUser(t, pool, "usr_b")
		test(t, review.NewPostgresStore(pool, maxAttempts))
	})
}

func pr(user string) review.Request {
	return review.Request{UserID: user, Source: review.Source{Repo: "o/r", Base: "main", Head: "feature"}}
}

func TestIdempotency(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		req := pr("usr_a")
		req.IdempotencyKey = "k"
		first, created, err := s.Create(ctx, req)
		if err != nil || !created {
			t.Fatalf("first create: %v %v", created, err)
		}
		again, created, err := s.Create(ctx, req)
		if err != nil || created || again.ID != first.ID {
			t.Fatalf("replay: id %s created %v err %v", again.ID, created, err)
		}
		other := pr("usr_b")
		other.IdempotencyKey = "k"
		if _, created, err := s.Create(ctx, other); err != nil || !created {
			t.Fatalf("same key, other user: %v %v", created, err)
		}
		req.Note = "different"
		if _, _, err := s.Create(ctx, req); !errors.Is(err, review.ErrIdempotencyConflict) {
			t.Fatalf("conflict: got %v", err)
		}
	})
}

func TestOwnership(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		r, _, err := s.Create(ctx, pr("usr_a"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.Get(ctx, "usr_b", r.ID); !errors.Is(err, review.ErrNotFound) {
			t.Fatalf("other user get: %v", err)
		}
		if err := s.Delete(ctx, "usr_b", r.ID); !errors.Is(err, review.ErrNotFound) {
			t.Fatalf("other user delete: %v", err)
		}
		if list, _ := s.List(ctx, "usr_b", 10); len(list) != 0 {
			t.Fatalf("other user list: %v", list)
		}
	})
}

func TestFencing(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr("usr_a"))
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
		got, _ := s.Get(ctx, "usr_a", r.ID)
		if got.Status != review.StatusDone || got.Result != review.ResultComplete || got.Attempts != 2 {
			t.Fatalf("final: %+v", got)
		}
	})
}

func TestReap(t *testing.T) {
	forEachStore(t, 1, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr("usr_a"))
		if _, _, err := s.Claim(ctx, "a", 10*time.Millisecond); err != nil {
			t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
		if _, err := s.Reap(ctx); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Get(ctx, "usr_a", r.ID); got.Status != review.StatusDone || got.Result != review.ResultNone || got.Reason == "" {
			t.Fatalf("got %+v, want done with no result and a reason", got)
		}
	})
}

func TestFinish(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr("usr_a"))
		_, l, _ := s.Claim(ctx, "a", time.Minute)
		if err := s.Finish(ctx, l, review.Outcome{Result: review.ResultNone}); err == nil {
			t.Fatal("finish with no result and no reason must fail")
		}
		if err := s.Finish(ctx, l, review.Outcome{Result: review.ResultNone, Reason: "clone failed"}); err != nil {
			t.Fatal(err)
		}
		got, _ := s.Get(ctx, "usr_a", r.ID)
		if got.Status != review.StatusDone || got.Result != review.ResultNone || got.Reason != "clone failed" || got.FinishedAt.IsZero() {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestRelease(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		r, _, _ := s.Create(ctx, pr("usr_a"))
		_, l, _ := s.Claim(ctx, "a", time.Minute)
		if err := s.Release(ctx, l); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Get(ctx, "usr_a", r.ID); got.Status != review.StatusQueued || got.Attempts != 0 {
			t.Fatalf("got %+v", got)
		}
	})
}

func TestDelete(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		queued, _, _ := s.Create(ctx, pr("usr_a"))
		if err := s.Delete(ctx, "usr_a", queued.ID); err != nil {
			t.Fatalf("delete queued: %v", err)
		}
		if _, _, err := s.Claim(ctx, "a", time.Minute); !errors.Is(err, review.ErrQueueEmpty) {
			t.Fatalf("claim after delete: %v", err)
		}

		r, _, _ := s.Create(ctx, pr("usr_a"))
		_, l, _ := s.Claim(ctx, "a", time.Minute)
		if err := s.Delete(ctx, "usr_a", r.ID); !errors.Is(err, review.ErrRunning) {
			t.Fatalf("delete running: %v", err)
		}
		if err := s.Finish(ctx, l, review.Outcome{Result: review.ResultComplete}); err != nil {
			t.Fatal(err)
		}
		if err := s.Delete(ctx, "usr_a", r.ID); err != nil {
			t.Fatalf("delete done: %v", err)
		}
		if _, err := s.Get(ctx, "usr_a", r.ID); !errors.Is(err, review.ErrNotFound) {
			t.Fatalf("get after delete: %v", err)
		}
	})
}

func TestDeleteRacesClaim(t *testing.T) {
	forEachStore(t, 3, func(t *testing.T, s store) {
		for range 20 {
			r, _, _ := s.Create(ctx, pr("usr_a"))
			var claimErr, deleteErr error
			var wg sync.WaitGroup
			wg.Go(func() { _, _, claimErr = s.Claim(ctx, "a", time.Minute) })
			wg.Go(func() { deleteErr = s.Delete(ctx, "usr_a", r.ID) })
			wg.Wait()
			switch {
			case claimErr == nil && errors.Is(deleteErr, review.ErrRunning):
				got, err := s.Get(ctx, "usr_a", r.ID)
				if err != nil || got.Status != review.StatusRunning {
					t.Fatalf("claim won but review is %+v (%v)", got, err)
				}
				if err := s.Delete(ctx, "usr_a", r.ID); !errors.Is(err, review.ErrRunning) {
					t.Fatalf("running review deleted: %v", err)
				}
			case errors.Is(claimErr, review.ErrQueueEmpty) && deleteErr == nil:
			default:
				t.Fatalf("claim %v, delete %v: exactly one must win", claimErr, deleteErr)
			}
			for {
				_, l, err := s.Claim(ctx, "drain", time.Minute)
				if errors.Is(err, review.ErrQueueEmpty) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := s.Finish(ctx, l, review.Outcome{Result: review.ResultComplete}); err != nil {
					t.Fatal(err)
				}
			}
		}
	})
}
