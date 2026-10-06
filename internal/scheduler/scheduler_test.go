package scheduler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/review/reviewtest"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

func opts() Options {
	return Options{Owner: "test", Workers: 2, LeaseTTL: 30 * time.Millisecond, Poll: 5 * time.Millisecond}
}

func create(t *testing.T, s *reviewtest.MemoryStore, n int) []string {
	t.Helper()
	ids := make([]string, 0, n)
	for i := range n {
		r, _, err := s.Create(context.Background(), review.Request{UserID: "usr_a", Source: review.Source{Repo: "o/r", Base: "main", Head: fmt.Sprintf("feature-%d", i)}})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, r.ID)
	}
	return ids
}

func waitFor(t *testing.T, s *reviewtest.MemoryStore, ids []string) map[string]review.Review {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		out := map[string]review.Review{}
		for _, id := range ids {
			r, _ := s.Get(context.Background(), "usr_a", id)
			if r.Status == review.StatusDone {
				out[id] = r
			}
		}
		if len(out) == len(ids) {
			return out
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("reviews did not finish in time")
	return nil
}

func TestRun(t *testing.T) {
	s := reviewtest.NewMemoryStore(3)
	ids := create(t, s, 5)
	fail := ids[2]
	exec := ExecutorFunc(func(_ context.Context, r review.Review) error {
		if r.ID == fail {
			return errors.New("boom")
		}
		return nil
	})
	ctx := t.Context()
	go NewWorkerPool(quiet, s, exec, opts()).Run(ctx)

	for id, r := range waitFor(t, s, ids) {
		want, reason := review.ResultComplete, ""
		if id == fail {
			want, reason = review.ResultNone, "boom"
		}
		if r.Result != want || r.Reason != reason {
			t.Errorf("%s: %s %q, want %s %q", id, r.Result, r.Reason, want, reason)
		}
	}
}

func TestShutdown(t *testing.T) {
	s := reviewtest.NewMemoryStore(3)
	ids := create(t, s, 1)
	started := make(chan struct{})
	block := ExecutorFunc(func(ctx context.Context, _ review.Review) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { NewWorkerPool(quiet, s, block, opts()).Run(ctx); close(done) }()
	<-started
	cancel()
	<-done

	if r, _ := s.Get(context.Background(), "usr_a", ids[0]); r.Status != review.StatusQueued {
		t.Fatalf("after shutdown: %s, want queued", r.Status)
	}
	ctx2 := t.Context()
	ok := ExecutorFunc(func(context.Context, review.Review) error { return nil })
	go NewWorkerPool(quiet, s, ok, opts()).Run(ctx2)
	if r := waitFor(t, s, ids)[ids[0]]; r.Result != review.ResultComplete || r.Attempts != 1 {
		t.Fatalf("second process: %+v", r)
	}
}
