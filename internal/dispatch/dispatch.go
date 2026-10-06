package dispatch

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func (f *ProviderError) Error() string { return fmt.Sprintf("%s: %v", f.Kind, f.Err) }
func (f *ProviderError) Unwrap() error { return f.Err }

func New(tiers map[workflow.Tier][]Candidate) *Dispatcher {
	d := &Dispatcher{
		now:       time.Now,
		rand:      rand.IntN,
		tiers:     tiers,
		cooldowns: map[string]cooldown{},
		lanes:     map[string]chan struct{}{},
	}
	for _, cs := range tiers {
		for _, c := range cs {
			if c.MaxConcurrency > 0 {
				d.lanes[c.ID] = make(chan struct{}, c.MaxConcurrency)
			}
		}
	}
	return d
}

func (d *Dispatcher) Pick(tier workflow.Tier, tried map[string]bool) (Candidate, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	candidates := d.tiers[tier]
	pool := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		if tried[c.ID] || now.Before(d.cooldowns[c.ID].until) {
			continue
		}
		if len(pool) > 0 && c.Priority > pool[0].Priority {
			continue
		}
		if len(pool) > 0 && c.Priority < pool[0].Priority {
			pool = pool[:0]
		}
		pool = append(pool, c)
	}
	if len(pool) == 0 {
		return Candidate{}, ErrNoCandidate
	}
	total := 0
	for _, c := range pool {
		total += max(c.Weight, 1)
	}
	n := d.rand(total)
	for _, c := range pool {
		n -= max(c.Weight, 1)
		if n < 0 {
			return c, nil
		}
	}
	return pool[len(pool)-1], nil
}

func (d *Dispatcher) Acquire(ctx context.Context, id string) (release func(), err error) {
	lane := d.lanes[id]
	if lane == nil {
		return func() {}, nil
	}
	select {
	case lane <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-lane }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *Dispatcher) Succeed(id string) {
	d.mu.Lock()
	delete(d.cooldowns, id)
	d.mu.Unlock()
}

func (d *Dispatcher) Fail(id string, f *ProviderError) time.Duration {
	d.mu.Lock()
	defer d.mu.Unlock()
	now := d.now()
	r := d.cooldowns[id]
	r.strikes++
	var length time.Duration
	switch f.Kind {
	case FailureCredit:
		length = creditCooldown
	case FailureQuota:
		length = min(max(f.ResetAt.Sub(now), baseCooldown), maxQuotaCooldown)
	case FailureRateLimit:
		length = max(f.RetryAfter, baseCooldown)
	case FailureBadRequest:
		return 0
	default:
		length = min(baseCooldown<<min(r.strikes-1, 5), maxCooldown)
	}
	r.until, r.kind = now.Add(length), f.Kind
	d.cooldowns[id] = r
	return length
}

func Decide(f *ProviderError, committed bool, attemptsLeft int) Decision {
	switch {
	case committed:
		return Decision{Reason: "output_committed"}
	case f.Kind == FailureBadRequest:
		return Decision{Reason: "non_retryable"}
	case attemptsLeft <= 0:
		return Decision{Reason: "attempts_exhausted"}
	default:
		return Decision{Retry: true, Reason: "candidate_failed:" + string(f.Kind)}
	}
}

func (d *Dispatcher) Run(ctx context.Context, tier workflow.Tier, maxAttempts int, call Call) ([]Attempt, error) {
	tried := map[string]bool{}
	var log []Attempt
	for left := maxAttempts - 1; ; left-- {
		c, err := d.Pick(tier, tried)
		if err != nil {
			return log, err
		}
		tried[c.ID] = true
		release, err := d.Acquire(ctx, c.ID)
		if err != nil {
			return log, err
		}
		committed, err := call(ctx, c)
		release()
		if err == nil {
			d.Succeed(c.ID)
			return append(log, Attempt{Candidate: c}), nil
		}
		var f *ProviderError
		if !errors.As(err, &f) {
			f = &ProviderError{Kind: FailureUnclassified, Err: err}
		}
		a := Attempt{Candidate: c, Err: err, Cooldown: d.Fail(c.ID, f), Decision: Decide(f, committed, left)}
		log = append(log, a)
		if !a.Decision.Retry || ctx.Err() != nil {
			return log, err
		}
	}
}
