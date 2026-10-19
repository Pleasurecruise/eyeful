package harness

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/sandbox"
	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

type State string

const (
	StateRunning State = "running"
	StateDone    State = "done"
	StateFailed  State = "failed"
)

type Status struct {
	State   State            `json:"state"`
	Outcome workflow.Outcome `json:"outcome"`
	Reason  string           `json:"reason"`
}

type Host interface {
	Token(ctx context.Context, installation int64) (string, error)
	Wait(ctx context.Context, token string, src review.Source) (github.Actions, error)
	Description(ctx context.Context, token string, src review.Source) (string, error)
	React(ctx context.Context, token string, src review.Source) error
	Publish(ctx context.Context, src review.Source, id, markdown string, notes []github.Note) error
}

type Runner struct {
	ID         string
	Log        *slog.Logger
	Experts    []string
	Classifier workflow.Classifier
	Dir        string
	Runs       workflow.Archive
	Sandbox    *sandbox.Client
	Host       Host
	Clone      func(ctx context.Context, dir, token string, src review.Source) error
}

func (r Runner) Execute(ctx context.Context, src review.Source) (Status, error) {
	published, ok, err := r.Runs.Load(ctx, "published")
	if err != nil {
		return Status{}, fmt.Errorf("publication record: %w", err)
	}
	if ok {
		return Status{State: StateDone, Outcome: workflow.Outcome(published)}, nil
	}
	token, err := r.Host.Token(ctx, src.InstallationID)
	if err != nil {
		return Status{}, fmt.Errorf("installation token: %w", err)
	}
	checkout := filepath.Join(r.Dir, "checkout")
	if err := r.Clone(ctx, checkout, token, src); err != nil {
		return Status{}, fmt.Errorf("checkout: %w", err)
	}
	actions, err := r.Host.Wait(ctx, token, src)
	if err != nil {
		return Status{}, fmt.Errorf("actions: %w", err)
	}
	if err := r.Host.React(ctx, token, src); err != nil {
		return Status{}, fmt.Errorf("reaction: %w", err)
	}
	description, err := r.Host.Description(ctx, token, src)
	if err != nil {
		return Status{}, fmt.Errorf("pull request: %w", err)
	}
	cfg, err := r.Sandbox.Open(ctx, src)
	if err != nil {
		return Status{}, fmt.Errorf("sandbox: %w", err)
	}
	res, err := app.Review(ctx, app.Options{
		Dir: checkout, Range: subject.Range{Base: src.BaseSHA, Head: src.HeadSHA}, Experts: r.Experts,
		Description: description, Verification: workflow.ModeExecution, Commands: cfg, Run: filepath.Join(r.Dir, "run"),
		Classifier: r.Classifier, Workspace: workspace{Client: r.Sandbox, check: workflow.Check{Passed: actions.Passed, Log: actions.Log}}, Archive: r.Runs,
		Out: io.Discard, Err: io.Discard, Log: r.Log,
	})
	if err != nil {
		return Status{}, fmt.Errorf("review: %w", err)
	}
	if res.Outcome == "" {
		return Status{State: StateDone, Reason: "the change is empty"}, nil
	}
	markdown, err := report.Summary(res)
	if err != nil {
		return Status{}, fmt.Errorf("report: %w", err)
	}
	if err := r.Host.Publish(ctx, src, r.ID, markdown, notesFrom(res.Comments)); err != nil {
		return Status{}, fmt.Errorf("publish: %w", err)
	}
	if err := r.Runs.Save(ctx, "published", []byte(res.Outcome)); err != nil {
		return Status{}, fmt.Errorf("record publication: %w", err)
	}
	r.Log.Info("published", slog.String("repo", src.Repo), slog.Int64("pull", src.Number), slog.String("outcome", string(res.Outcome)))
	return Status{State: StateDone, Outcome: res.Outcome, Reason: res.Reason}, nil
}

type workspace struct {
	*sandbox.Client
	check workflow.Check
}

func (w workspace) CI(context.Context) (workflow.Check, error) { return w.check, nil }

func notesFrom(comments []workflow.Comment) []github.Note {
	notes := []github.Note{}
	for _, c := range comments {
		if c.Path == "" || c.Line < 1 || !report.Posted(c.Label, c.Kind) {
			continue
		}
		var b strings.Builder
		b.WriteString(string(c.Label))
		if len(c.Decorations) > 0 {
			b.WriteString(" (" + strings.Join(c.Decorations, ", ") + ")")
		}
		b.WriteString(": " + c.Subject)
		for _, part := range []string{c.Discussion, c.Scenario, c.Objection} {
			if part != "" {
				b.WriteString("\n\n" + part)
			}
		}
		if len(c.Fix) == 1 && c.Fix[0].Path == c.Path && c.Fix[0].New != "" {
			b.WriteString("\n\n```suggestion\n" + c.Fix[0].New)
			if !strings.HasSuffix(c.Fix[0].New, "\n") {
				b.WriteByte('\n')
			}
			b.WriteString("```\n")
		}
		notes = append(notes, github.Note{Path: c.Path, Line: c.Line, Body: b.String()})
	}
	return notes
}
