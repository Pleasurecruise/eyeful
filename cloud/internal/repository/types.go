package repository

import (
	"context"
	"errors"
	"time"

	"golang.org/x/oauth2"
)

type Tokens interface {
	Token(ctx context.Context, userID string) (*oauth2.Token, error)
}

type Service struct {
	tokens  Tokens
	apiBase string
}

type RateLimitedError struct {
	RetryAfter time.Duration
	Err        error
}

var (
	ErrInvalidName = errors.New("invalid repository, ref or sha")
	ErrNotFound    = errors.New("not among the repositories this user gave eyeful")
)
