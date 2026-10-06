package worktree

import (
	"errors"
	"log/slog"
	"time"

	"github.com/Pleasurecruise/eyeful/workflow/project"
)

type Worktree struct {
	root   string
	dir    string
	env    []string
	change string
	log    *slog.Logger
}

type Workspace struct {
	tree     *Worktree
	cfg      project.Config
	timeout  time.Duration
	turn     chan struct{}
	setup    bool
	setupErr error
}

const outputLimit = 64 << 10

var (
	ErrSetup        = errors.New("setup failed")
	ErrNoCommand    = errors.New("no command configured")
	ErrInvalidEdit  = errors.New("invalid edit")
	ErrUnsafeTarget = errors.New("path outside the repository")
)
