package subject

import "errors"

type Range struct {
	Base string
	Head string
}

type Snapshot struct {
	Root   string `json:"root"`
	Base   string `json:"base"`
	Head   string `json:"head"`
	Commit string `json:"commit"`
	Patch  string `json:"-"`
}

type Branch struct {
	Name     string `json:"name"`
	Current  bool   `json:"current"`
	Worktree string `json:"worktree"`
}

const (
	EmptyTree  = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	maxCommits = 50
)

var (
	ErrNotRepository = errors.New("not a git repository")
	ErrUnknownRef    = errors.New("unknown ref")
	ErrNoMergeBase   = errors.New("no common history")
	ErrNotCheckedOut = errors.New("the change under review is not the checked-out branch")
)
