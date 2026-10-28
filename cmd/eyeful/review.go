package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/classify"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

type reviewCmd struct {
	Experts      []string `arg:"" optional:"" help:"Experts to review with alone, such as security or correctness; none lets the planner choose."`
	Base         string   `help:"Branch or commit to compare against; the merge base with the head is used."`
	Head         string   `help:"Commit or branch to review; empty reviews uncommitted changes."`
	Verification string   `enum:"execution,read_only,none" default:"execution" help:"How findings are checked: run their tests in a checkout of the change, a judge agent, or not at all."`
	Setup        string   `help:"Command that installs the project's dependencies, in place of the one eyeful finds."`
	Lint         string   `help:"Command that lints the project, in place of the one eyeful finds."`
	Test         string   `help:"Command that runs the project's tests, in place of the one eyeful finds."`
	TestOne      string   `help:"Command that runs one test; {test} is replaced by its identifier. In place of the one eyeful finds."`
	Detect       bool     `default:"true" negatable:"" help:"Find the project's commands from its manifests, such as go.mod or package.json, when none is given."`
	Description  string   `type:"existingfile" help:"A file with the author's description of the change, such as a pull request's title and body; the reviewers read it as the author's word."`
	Archive      string   `name:"run" type:"path" help:"Directory for this review's files; stages already saved there are not run again. Empty makes one under .eyeful/runs."`
	Jev          string   `name:"typesafe-api-key" env:"EYEFUL_TYPESAFE_API_KEY" help:"TypeSafe API key for Jev, which says which findings on the same lines are one problem. Without it they are not merged."`
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
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	_, err := app.Review(ctx, app.Options{
		Dir: ".", Range: subject.Range{Base: c.Base, Head: c.Head}, Experts: c.Experts, Verification: workflow.Mode(c.Verification), Run: c.Archive,
		Commands: project.Config{Setup: c.Setup, Lint: c.Lint, Test: c.Test, TestOne: c.TestOne}, Detect: c.Detect, Description: string(description),
		Classifier: classify.New(log, c.Jev),
		Confirm: func(context.Context, string, []project.Command) (bool, error) {
			if c.Yes {
				return true, nil
			}
			return ask("Run these commands?")
		},
		Out: os.Stdout, Err: os.Stderr, Log: log,
	})
	return err
}
