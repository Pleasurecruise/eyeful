package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Pleasurecruise/eyeful/local/agents"
	"github.com/Pleasurecruise/eyeful/local/git"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
)

func CommitMessage(ctx context.Context, dir string, log *slog.Logger) (string, error) {
	objects, err := os.MkdirTemp("", "eyeful-snapshot-")
	if err != nil {
		return "", fmt.Errorf("snapshot objects: %w", err)
	}
	snap, err := subject.Read(ctx, dir, subject.Range{}, objects)
	if err := errors.Join(err, os.RemoveAll(objects)); err != nil {
		return "", err
	}
	if snap.Patch == "" {
		return "", ErrNothingToCommit
	}
	provider, _, err := connected(ctx)
	if err != nil {
		return "", err
	}
	set, err := prompts.Load()
	if err != nil {
		return "", err
	}
	message, _, err := agents.New(log, agents.Config{Provider: provider, Dir: snap.Root, Set: set}, idleTimeout, hardTimeout).CommitMessage(ctx, snap.Patch)
	return strings.TrimSpace(message), err
}

func Commit(ctx context.Context, dir, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", ErrNoMessage
	}
	if _, err := git.Run(ctx, dir, nil, "add", "--all"); err != nil {
		return "", err
	}
	if _, err := git.Run(ctx, dir, nil, "commit", "--quiet", "--message", message); err != nil {
		return "", err
	}
	out, err := git.Run(ctx, dir, nil, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}
