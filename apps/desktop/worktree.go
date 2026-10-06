package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/subject"
)

func (w *Worktree) root() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dir
}

func (w *Worktree) Open() (string, error) {
	dir, err := application.Get().Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("choose a repository: %w", err)
	}
	if dir == "" {
		return "", nil
	}
	return dir, w.Use(dir)
}

func (w *Worktree) Use(dir string) error {
	w.mu.Lock()
	w.dir = dir
	w.mu.Unlock()
	return app.Remember(dir)
}

func (w *Worktree) Branches(ctx context.Context) ([]subject.Branch, error) {
	return subject.Branches(ctx, w.root())
}

func (w *Worktree) Changes(ctx context.Context, base, head string) (Changes, error) {
	objects, err := os.MkdirTemp("", "eyeful-snapshot-")
	if err != nil {
		return Changes{}, fmt.Errorf("snapshot objects: %w", err)
	}
	s, err := subject.Read(ctx, w.root(), subject.Range{Base: base, Head: head}, objects)
	if err := errors.Join(err, os.RemoveAll(objects)); err != nil {
		return Changes{}, err
	}
	return Changes{Root: s.Root, Base: s.Base, Patch: s.Patch}, nil
}
