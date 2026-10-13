package dispatch

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Pleasurecruise/eyeful/workflow"
)

// TODO(runtime): run every pi agent through Dispatcher.Run, with tiers loaded from config.
// TODO(scheduling): per-review affinity so one role keeps one candidate within a review.
// TODO(scheduling): classify provider HTTP errors into Failure kinds.
type Candidate struct {
	ID             string
	Model          string
	Priority       int
	Weight         int
	MaxConcurrency int
}

type FailureKind string

const (
	FailureCredit       FailureKind = "credit"
	FailureQuota        FailureKind = "quota"
	FailureRateLimit    FailureKind = "rate_limit"
	FailureUnavailable  FailureKind = "unavailable"
	FailureBadRequest   FailureKind = "bad_request"
	FailureUnclassified FailureKind = "unclassified"
)

type ProviderError struct {
	Kind       FailureKind
	RetryAfter time.Duration
	ResetAt    time.Time
	Err        error
}

type Decision struct {
	Retry  bool
	Reason string
}

var ErrNoCandidate = errors.New("no available candidate")

const (
	creditCooldown   = 30 * time.Minute
	baseCooldown     = time.Minute
	maxCooldown      = 30 * time.Minute
	maxQuotaCooldown = 8 * 24 * time.Hour
)

type Attempt struct {
	Candidate Candidate
	Decision  Decision
	Cooldown  time.Duration
	Err       error
}

type Call func(ctx context.Context, c Candidate) (committed bool, err error)

type cooldown struct {
	until   time.Time
	strikes int
	kind    FailureKind
}

type Dispatcher struct {
	mu        sync.Mutex
	now       func() time.Time
	rand      func(int) int
	tiers     map[workflow.Tier][]Candidate
	cooldowns map[string]cooldown
	lanes     map[string]chan struct{}
}
