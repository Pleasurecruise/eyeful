package classify

import (
	"errors"
	"log/slog"

	jev "github.com/mattn/go-jev"
)

type Jev struct {
	log    *slog.Logger
	client *jev.Client
}

type finding struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Subject    string `json:"subject"`
	Discussion string `json:"discussion"`
}

const (
	sameQuestion = "Two code reviewers each wrote a finding about the same change. Do `findings.%s` and `findings.%s` report the same underlying problem, so that one fix would resolve both? Answer no when they are different problems that only sit on nearby lines."
)

var (
	ErrNoKey      = errors.New("Jev needs a TypeSafe API key")
	ErrRejected   = errors.New("jev rejected the request")
	ErrUnanswered = errors.New("jev left a question unanswered")
)
