package reviewtest

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

func NewMemoryStore(maxAttempts int32) *MemoryStore {
	return &MemoryStore{
		now:         time.Now,
		maxAttempts: max(maxAttempts, 1),
		records:     map[string]*record{},
		byKey:       map[string]string{},
	}
}

func (s *MemoryStore) Create(_ context.Context, req review.Request) (review.Review, bool, error) {
	if err := req.Validate(); err != nil {
		return review.Review{}, false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := req.UserID + "\x00" + req.IdempotencyKey
	if req.IdempotencyKey != "" {
		if id, ok := s.byKey[key]; ok {
			prev := s.records[id]
			if prev.request != req {
				return review.Review{}, false, review.ErrIdempotencyConflict
			}
			return prev.Review, false, nil
		}
	}
	r := &record{
		ID:        review.NewID(),
		UserID:    req.UserID,
		Source:    req.Source,
		Note:      req.Note,
		Status:    review.StatusQueued,
		CreatedAt: s.now(),
		request:   req,
	}
	s.records[r.ID] = r
	s.order = append(s.order, r.ID)
	if req.IdempotencyKey != "" {
		s.byKey[key] = r.ID
	}
	return r.Review, true, nil
}

func (s *MemoryStore) Get(_ context.Context, userID, id string) (review.Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	if !ok || r.UserID != userID {
		return review.Review{}, review.ErrNotFound
	}
	return r.Review, nil
}

func (s *MemoryStore) List(_ context.Context, userID string, limit int32) ([]review.Review, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []review.Review{}
	for _, id := range slices.Backward(s.order) {
		if len(out) == int(limit) {
			break
		}
		if r := s.records[id]; r.UserID == userID {
			out = append(out, r.Review)
		}
	}
	return out, nil
}

func (s *MemoryStore) Claim(_ context.Context, owner string, ttl time.Duration) (review.Review, review.Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range s.order {
		r := s.records[id]
		if r.Status != review.StatusQueued {
			continue
		}
		s.token++
		r.lease = review.Lease{ReviewID: id, Owner: owner, FencingToken: s.token}
		r.leaseUntil = s.now().Add(ttl)
		r.Status = review.StatusRunning
		r.Attempts++
		if r.StartedAt.IsZero() {
			r.StartedAt = s.now()
		}
		return r.Review, r.lease, nil
	}
	return review.Review{}, review.Lease{}, review.ErrQueueEmpty
}

func (s *MemoryStore) Renew(_ context.Context, l review.Lease, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.held(l)
	if err != nil {
		return err
	}
	r.leaseUntil = s.now().Add(ttl)
	return nil
}

func (s *MemoryStore) Finish(_ context.Context, l review.Lease, o review.Outcome) error {
	if err := o.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.held(l)
	if err != nil {
		return err
	}
	r.Status, r.Result, r.Reason, r.FinishedAt = review.StatusDone, o.Result, o.Reason, s.now()
	r.lease = review.Lease{}
	return nil
}

func (s *MemoryStore) Release(_ context.Context, l review.Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.held(l)
	if err != nil {
		return err
	}
	r.Status, r.lease = review.StatusQueued, review.Lease{}
	r.Attempts--
	return nil
}

func (s *MemoryStore) Reap(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now, n := s.now(), 0
	for _, r := range s.records {
		if r.Status != review.StatusRunning || now.Before(r.leaseUntil) {
			continue
		}
		n++
		r.lease = review.Lease{}
		if r.Attempts >= s.maxAttempts {
			r.Status, r.Result, r.FinishedAt = review.StatusDone, review.ResultNone, now
			r.Reason = fmt.Sprintf("worker lost %d times", r.Attempts)
		} else {
			r.Status = review.StatusQueued
		}
	}
	return n, nil
}

func (s *MemoryStore) held(l review.Lease) (*record, error) {
	r, ok := s.records[l.ReviewID]
	if !ok {
		return nil, review.ErrNotFound
	}
	if r.lease.FencingToken != l.FencingToken || r.Status != review.StatusRunning {
		return nil, review.ErrFenced
	}
	return r, nil
}

func (s *MemoryStore) Delete(_ context.Context, userID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.records[id]
	switch {
	case !ok || r.UserID != userID:
		return review.ErrNotFound
	case r.Status == review.StatusRunning:
		return review.ErrRunning
	}
	delete(s.records, id)
	s.order = slices.DeleteFunc(s.order, func(o string) bool { return o == id })
	for key, rid := range s.byKey {
		if rid == id {
			delete(s.byKey, key)
		}
	}
	return nil
}
