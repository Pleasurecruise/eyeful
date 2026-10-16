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
	Repo           string `json:"repo"`
	Base           string `json:"base"`
	Head           string `json:"head"`
	Number         int64  `json:"number,omitempty"`
	BaseSHA        string `json:"base_sha,omitempty"`
	HeadSHA        string `json:"head_sha,omitempty"`
	CloneURL       string `json:"clone_url,omitempty"`
	InstallationID int64  `json:"installation_id,omitempty"`
}

var (
	ErrNotFound            = errors.New("review not found")
	ErrIdempotencyConflict = errors.New("idempotency key reused with different input")
	ErrInvalidSource       = errors.New("invalid source")
	ErrQueueEmpty          = errors.New("no queued review")
	ErrFenced              = errors.New("lease superseded")
)

type Request struct {
	Source         Source
	IdempotencyKey string
}

type Review struct {
	ID         string
	Source     Source
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
