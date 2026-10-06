package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

type reviewCmd struct {
	Experts      []string `arg:"" optional:"" help:"Experts to review with alone, such as security or correctness; none lets the planner choose."`
	Base         string   `help:"Branch or commit to compare against; the merge base with the head is used."`
	Head         string   `help:"Commit or branch to review; empty reviews uncommitted changes."`
	Verification string   `enum:"execution,read_only,none" default:"read_only" help:"How findings are checked: run their tests in a checkout of the change, a judge agent, or not at all."`
	Yes          bool     `short:"y" help:"Confirm the project commands without asking."`
}

func (c *reviewCmd) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	_, err := app.Review(ctx, app.Options{
		Dir: ".", Range: subject.Range{Base: c.Base, Head: c.Head}, Experts: c.Experts, Verification: workflow.Mode(c.Verification),
		Confirm: func(context.Context, string, []project.Command) (bool, error) {
			if c.Yes {
				return true, nil
			}
			return ask("Run these commands?")
		},
		Out: os.Stdout, Err: os.Stderr, Log: slog.New(slog.NewTextHandler(os.Stderr, nil)),
	})
	return err
}
