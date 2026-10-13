package main

import (
	"context"
	"fmt"
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
	Description  string   `type:"existingfile" help:"A file with the author's description of the change, such as a pull request's title and body; the reviewers read it as the author's word."`
	Archive      string   `name:"run" type:"path" help:"Directory for this review's files; stages already saved there are not run again. Empty makes one under .eyeful/runs."`
	TypesafeKey  string   `env:"EYEFUL_TYPESAFE_API_KEY" help:"TypeSafe API key; Jev says which findings on the same lines are one problem. Without it they are not merged."`
	Yes          bool     `short:"y" help:"Confirm the project commands without asking."`
}

func (c *reviewCmd) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var description []byte
	if c.Description != "" {
		var err error
		if description, err = os.ReadFile(c.Description); err != nil {
			return fmt.Errorf("read the description: %w", err)
		}
	}
	_, err := app.Review(ctx, app.Options{
		Dir: ".", Range: subject.Range{Base: c.Base, Head: c.Head}, Experts: c.Experts, Verification: workflow.Mode(c.Verification), Run: c.Archive, Description: string(description), TypesafeKey: c.TypesafeKey,
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
