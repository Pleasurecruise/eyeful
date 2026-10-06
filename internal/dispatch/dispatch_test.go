package dispatch

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func fixed(tiers map[workflow.Tier][]Candidate) (*Dispatcher, *time.Time) {
	d := New(tiers)
	now := time.Unix(0, 0)
	d.now = func() time.Time { return now }
	d.rand = func(int) int { return 0 }
	return d, &now
}

var strong = map[workflow.Tier][]Candidate{"strong": {
	{ID: "a", Priority: 0},
	{ID: "b", Priority: 0},
	{ID: "c", Priority: 1},
}}

func TestRun(t *testing.T) {
	d, _ := fixed(strong)
	var order []string
	attempts, err := d.Run(context.Background(), "strong", 3, func(_ context.Context, c Candidate) (bool, error) {
		order = append(order, c.ID)
		if c.ID != "c" {
			return false, &ProviderError{Kind: FailureUnavailable, Err: errors.New("503")}
		}
		return false, nil
	})
	if err != nil || len(attempts) != 3 {
		t.Fatalf("attempts %+v err %v", attempts, err)
	}
	if order[2] != "c" {
		t.Fatalf("order %v, want priority 1 last", order)
	}
	if !attempts[0].Decision.Retry || attempts[0].Decision.Reason != "candidate_failed:unavailable" {
		t.Errorf("decision %+v", attempts[0].Decision)
	}
}

func TestRunCommitted(t *testing.T) {
	d, _ := fixed(strong)
	calls := 0
	attempts, err := d.Run(context.Background(), "strong", 3, func(context.Context, Candidate) (bool, error) {
		calls++
		return true, &ProviderError{Kind: FailureUnavailable, Err: errors.New("stream cut")}
	})
	if err == nil || calls != 1 || attempts[0].Decision.Reason != "output_committed" {
		t.Fatalf("calls %d attempts %+v", calls, attempts)
	}
}

func TestFail(t *testing.T) {
	d, now := fixed(strong)
	cases := []struct {
		f    *ProviderError
		want time.Duration
	}{
		{&ProviderError{Kind: FailureCredit}, 30 * time.Minute},
		{&ProviderError{Kind: FailureQuota, ResetAt: now.Add(5 * time.Hour)}, 5 * time.Hour},
		{&ProviderError{Kind: FailureRateLimit, RetryAfter: 90 * time.Second}, 90 * time.Second},
		{&ProviderError{Kind: FailureBadRequest}, 0},
	}
	for _, c := range cases {
		d.Succeed("a")
		if got := d.Fail("a", c.f); got != c.want {
			t.Errorf("%s: cooldown %v, want %v", c.f.Kind, got, c.want)
		}
	}
	d.Succeed("a")
	got := make([]time.Duration, 0, 3)
	for range 3 {
		got = append(got, d.Fail("a", &ProviderError{Kind: FailureUnavailable}))
	}
	if got[0] != time.Minute || got[1] != 2*time.Minute || got[2] != 4*time.Minute {
		t.Errorf("repeated failures cooldown %v, want doubling from 1m", got)
	}
}

func TestPickCooldown(t *testing.T) {
	d, now := fixed(map[workflow.Tier][]Candidate{"t": {{ID: "a"}}})
	d.Fail("a", &ProviderError{Kind: FailureRateLimit, RetryAfter: time.Minute})
	if _, err := d.Pick("t", nil); !errors.Is(err, ErrNoCandidate) {
		t.Fatalf("pick during cooldown: %v", err)
	}
	*now = now.Add(time.Minute)
	if c, err := d.Pick("t", nil); err != nil || c.ID != "a" {
		t.Fatalf("pick after cooldown: %v %v", c, err)
	}
}

func TestPickWeight(t *testing.T) {
	d, _ := fixed(map[workflow.Tier][]Candidate{"t": {{ID: "a", Weight: 1}, {ID: "b", Weight: 3}}})
	for n, want := range map[int]string{0: "a", 1: "b", 3: "b"} {
		d.rand = func(int) int { return n }
		if c, _ := d.Pick("t", nil); c.ID != want {
			t.Errorf("rand %d: %s, want %s", n, c.ID, want)
		}
	}
}

func TestAcquire(t *testing.T) {
	d, _ := fixed(map[workflow.Tier][]Candidate{"t": {{ID: "a", MaxConcurrency: 1}, {ID: "b", Priority: 1}}})
	hold, err := d.Acquire(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan []Attempt)
	go func() {
		attempts, _ := d.Run(context.Background(), "t", 2, func(context.Context, Candidate) (bool, error) { return false, nil })
		done <- attempts
	}()
	select {
	case <-done:
		t.Fatal("run went ahead while the lane was full")
	case <-time.After(20 * time.Millisecond):
	}
	hold()
	attempts := <-done
	if len(attempts) != 1 || attempts[0].Candidate.ID != "a" {
		t.Fatalf("attempts %+v, want a after waiting, no fallback to b", attempts)
	}
}
