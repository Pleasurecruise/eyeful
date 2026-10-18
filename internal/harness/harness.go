package harness

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/review"
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
	Publish(ctx context.Context, src review.Source, markdown string, notes []github.Note) error
}

type Runner struct {
	Log        *slog.Logger
	Experts    []string
	Classifier workflow.Classifier
	Progress   io.Writer
	Runs       workflow.Archive
	Host       Host
	Clone      func(ctx context.Context, dir, token string, src review.Source) error
}

var errNoSandbox = errors.New("the sandbox is not built, so project commands do not run")

func (r Runner) Execute(ctx context.Context, src review.Source) (Status, error) {
	published, ok, err := r.Runs.Load(ctx, "published")
	if err != nil {
		return Status{}, fmt.Errorf("publication record: %w", err)
	}
	if ok {
		return Status{State: StateDone, Outcome: workflow.Outcome(published)}, nil
	}
	dir, err := os.MkdirTemp("", "eyeful-review-")
	if err != nil {
		return Status{}, fmt.Errorf("review directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			r.Log.Warn("remove the review directory", slog.String("path", dir), slog.Any("error", err))
		}
	}()
	token, err := r.Host.Token(ctx, src.InstallationID)
	if err != nil {
		return Status{}, fmt.Errorf("installation token: %w", err)
	}
	checkout := filepath.Join(dir, "checkout")
	if err := r.Clone(ctx, checkout, token, src); err != nil {
		return Status{}, fmt.Errorf("checkout: %w", err)
	}
	actions, err := r.Host.Wait(ctx, token, src)
	if err != nil {
		return Status{}, fmt.Errorf("actions: %w", err)
	}
	description, err := r.Host.Description(ctx, token, src)
	if err != nil {
		return Status{}, fmt.Errorf("pull request: %w", err)
	}
	res, err := app.Review(ctx, app.Options{
		Dir: checkout, Range: subject.Range{Base: src.BaseSHA, Head: src.HeadSHA}, Experts: r.Experts,
		Description: description, Verification: workflow.ModeReadOnly, Run: filepath.Join(dir, "run"),
		Classifier: r.Classifier, Workspace: workspace{check: workflow.Check{Passed: actions.Passed, Log: actions.Log}}, Archive: r.Runs,
		Out: io.Discard, Err: r.Progress, Log: r.Log,
	})
	if err != nil {
		return Status{}, fmt.Errorf("review: %w", err)
	}
	if res.Outcome == "" {
		return Status{State: StateDone, Reason: "the change is empty"}, nil
	}
	markdown, err := report.Markdown(res)
	if err != nil {
		return Status{}, fmt.Errorf("report: %w", err)
	}
	if err := r.Host.Publish(ctx, src, markdown, notesFrom(res.Comments)); err != nil {
		return Status{}, fmt.Errorf("publish: %w", err)
	}
	if err := r.Runs.Save(ctx, "published", []byte(res.Outcome)); err != nil {
		return Status{}, fmt.Errorf("record publication: %w", err)
	}
	r.Log.Info("published", slog.String("repo", src.Repo), slog.Int64("pull", src.Number), slog.String("outcome", string(res.Outcome)))
	return Status{State: StateDone, Outcome: res.Outcome, Reason: res.Reason}, nil
}

type workspace struct {
	check workflow.Check
}

func (w workspace) CI(context.Context) (workflow.Check, error) { return w.check, nil }

func (workspace) Setup(context.Context) error { return nil }

func (workspace) Read(context.Context, string) (string, error) { return "", errNoSandbox }

func (workspace) Test(context.Context, []workflow.Edit, workflow.Target) (workflow.TestRun, error) {
	return workflow.TestRun{}, errNoSandbox
}

func notesFrom(comments []workflow.Comment) []github.Note {
	notes := []github.Note{}
	for _, c := range comments {
		if c.Path == "" || c.Line < 1 {
			continue
		}
		notes = append(notes, github.Note{Path: c.Path, Line: c.Line, Body: noteBody(c)})
	}
	return notes
}

func noteBody(c workflow.Comment) string {
	var b strings.Builder
	b.WriteString(string(c.Label))
	if len(c.Decorations) > 0 {
		b.WriteString(" (" + strings.Join(c.Decorations, ", ") + ")")
	}
	b.WriteString(": " + c.Subject)
	if c.Discussion != "" {
		b.WriteString("\n\n" + c.Discussion)
	}
	if c.Scenario != "" {
		b.WriteString("\n\n" + c.Scenario)
	}
	if c.Objection != "" {
		b.WriteString("\n\n" + c.Objection)
	}
	if len(c.Fix) == 1 && c.Fix[0].Path == c.Path && c.Fix[0].New != "" {
		b.WriteString("\n\n```suggestion\n" + c.Fix[0].New)
		if !strings.HasSuffix(c.Fix[0].New, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n")
	}
	return b.String()
}
