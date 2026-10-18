package sandbox

import (
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/Pleasurecruise/eyeful/local/worktree"
	"github.com/Pleasurecruise/eyeful/workflow"
)

type Server struct {
	log     *slog.Logger
	root    string
	timeout time.Duration
	mu      sync.Mutex
	tree    *worktree.Worktree
	ws      *worktree.Workspace
}

type Client struct {
	base   string
	client *http.Client
}

type test struct {
	Edits  []workflow.Edit `json:"edits"`
	Target workflow.Target `json:"target"`
}

var ErrNotOpen = errors.New("the sandbox has no checkout yet")
