package harness

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path/filepath"

	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/sandbox"
	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

type Status struct {
	Outcome workflow.Outcome
	Reason  string
}

type Host interface {
	Wait(ctx context.Context, src review.Source) (github.Actions, error)
	Description(ctx context.Context, src review.Source) (string, error)
	React(ctx context.Context, src review.Source) error
	Publish(ctx context.Context, src review.Source, blocks []string, notes []report.Note) error
}

type Runner struct {
	Log        *slog.Logger
	Experts    []string
	Classifier workflow.Classifier
	Dir        string
	Sandbox    *sandbox.Client
	Host       Host
	Clone      func(ctx context.Context, dir string, src review.Source) error
	Out        io.Writer
	Err        io.Writer
}

func (r Runner) Execute(ctx context.Context, src review.Source) (Status, error) {
	checkout := filepath.Join(r.Dir, "checkout")
	if err := r.Clone(ctx, checkout, src); err != nil {
		return Status{}, fmt.Errorf("checkout: %w", err)
	}
	actions, err := r.Host.Wait(ctx, src)
	if err != nil {
		return Status{}, fmt.Errorf("actions: %w", err)
	}
	if err := r.Host.React(ctx, src); err != nil {
		return Status{}, fmt.Errorf("reaction: %w", err)
	}
	description, err := r.Host.Description(ctx, src)
	if err != nil {
		return Status{}, fmt.Errorf("pull request: %w", err)
	}
	mirror := src
	mirror.CloneURL = (&url.URL{Scheme: "file", Path: checkout}).String()
	cfg, err := r.Sandbox.Open(ctx, mirror)
	if err != nil {
		return Status{}, fmt.Errorf("sandbox: %w", err)
	}
	res, err := app.Review(ctx, app.Options{
		Dir: checkout, Range: subject.Range{Base: src.BaseSHA, Head: src.HeadSHA}, Experts: r.Experts,
		Description: description, Verification: workflow.ModeExecution, Commands: cfg, Run: filepath.Join(r.Dir, "run"),
		Classifier: r.Classifier, Workspace: workspace{Client: r.Sandbox, check: workflow.Check{Passed: actions.Passed, Log: actions.Log}},
		Out: r.Out, Err: r.Err, Log: r.Log,
	})
	if err != nil {
		return Status{}, fmt.Errorf("review: %w", err)
	}
	if res.Outcome == "" {
		return Status{Reason: "the change is empty"}, nil
	}
	blocks, err := report.Summary(res)
	if err != nil {
		return Status{}, fmt.Errorf("report: %w", err)
	}
	notes, err := report.Notes(res)
	if err != nil {
		return Status{}, fmt.Errorf("report: %w", err)
	}
	if err := r.Host.Publish(ctx, src, blocks, notes); err != nil {
		return Status{}, fmt.Errorf("publish: %w", err)
	}
	r.Log.Info("published", slog.String("repo", src.Repo), slog.Int64("pull", src.Number), slog.String("outcome", string(res.Outcome)))
	return Status{Outcome: res.Outcome, Reason: res.Reason}, nil
}

type workspace struct {
	*sandbox.Client
	check workflow.Check
}

func (w workspace) CI(context.Context) (workflow.Check, error) { return w.check, nil }
