package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

type Options struct {
	Dir          string
	Range        subject.Range
	Experts      []string
	Verification workflow.Mode
	Confirm      func(ctx context.Context, work string, commands []project.Command) (bool, error)
	Out          io.Writer
	Err          io.Writer
	Log          *slog.Logger
}

type Provider struct {
	agents.Provider
	Connected bool
	Installed bool
}

const (
	budget         = 3_000_000
	commandTimeout = 10 * time.Minute
	idleTimeout    = 2 * time.Minute
	hardTimeout    = 20 * time.Minute
)

var (
	ErrUnknownAgent    = errors.New("unknown agent; choose claude, codex or pi")
	ErrNotConfirmed    = errors.New("the project commands were not confirmed")
	ErrNothingToCommit = errors.New("nothing to commit: the working copy has no changes")
	ErrNoMessage       = errors.New("the commit message is empty")
)
