package review

import (
	"errors"
	"time"
)

type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusDone    Status = "done"
)

type Result string

const (
	ResultComplete Result = "complete"
	ResultNone     Result = "none"
)

type Source struct {
	Repo string `json:"repo"`
	Base string `json:"base"`
	Head string `json:"head"`
}

var (
	ErrNotFound            = errors.New("review not found")
	ErrRunning             = errors.New("review is running")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different input")
	ErrInvalidSource       = errors.New("invalid source")
	ErrQueueEmpty          = errors.New("no queued review")
	ErrFenced              = errors.New("lease superseded")
)

type Request struct {
	UserID         string
	Source         Source
	Note           string
	IdempotencyKey string
}

type Review struct {
	ID         string
	UserID     string
	Source     Source
	Note       string
	Status     Status
	Result     Result
	Reason     string
	Attempts   int32
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
}

type Lease struct {
	ReviewID     string
	Owner        string
	FencingToken int64
}

type Outcome struct {
	Result Result
	Reason string
}
