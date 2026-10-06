package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pleasurecruise/eyeful/local/app"
)

type commitCmd struct {
	Message string `short:"m" help:"Commit message; empty asks the connected agent to write one."`
	Yes     bool   `short:"y" help:"Commit without asking."`
}

func (c *commitCmd) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	message := c.Message
	if message == "" {
		var err error
		if message, err = app.CommitMessage(ctx, ".", slog.New(slog.NewTextHandler(os.Stderr, nil))); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(os.Stderr, "%s\n\n", message); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if !c.Yes {
		yes, err := ask("Commit every change with this message?")
		if err != nil {
			return fmt.Errorf("not committed: %w", err)
		}
		if !yes {
			return errors.New("not committed")
		}
	}
	head, err := app.Commit(ctx, ".", message)
	if err != nil {
		return err
	}
	if _, err := fmt.Printf("Committed %.12s.\n", head); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
