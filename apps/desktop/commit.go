package main

import (
	"context"
	"log/slog"

	"github.com/Pleasurecruise/eyeful/local/app"
)

func (c *Commit) Message(ctx context.Context) (string, error) {
	return app.CommitMessage(ctx, c.tree.root(), slog.New(slog.NewTextHandler(emitter(EventLog), nil)))
}

func (c *Commit) Create(ctx context.Context, message string) (string, error) {
	return app.Commit(ctx, c.tree.root(), message)
}
