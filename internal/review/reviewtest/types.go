package reviewtest

import (
	"sync"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

type MemoryStore struct {
	mu          sync.Mutex
	now         func() time.Time
	maxAttempts int32
	records     map[string]*record
	order       []string
	byKey       map[string]string
	token       int64
}

type record struct {
	review.Review
	request    review.Request
	lease      review.Lease
	leaseUntil time.Time
}
