package main

import (
	"context"
	"io"
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

func (e emitter) Write(p []byte) (int, error) {
	application.Get().Event.Emit(string(e), string(p))
	return len(p), nil
}

func (r *Review) Experts() ([]workflow.Expert, error) {
	return app.Experts()
}

func (r *Review) Start(ctx context.Context, base, head string, experts []string, verification workflow.Mode) (report.Output, error) {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return report.Output{}, ErrRunning
	}
	r.running = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.running = false
		r.mu.Unlock()
	}()
	log := emitter(EventLog)
	return app.Review(ctx, app.Options{
		Dir: r.tree.root(), Range: subject.Range{Base: base, Head: head}, Experts: experts, Verification: verification,
		Confirm: r.confirm, Out: io.Discard, Err: log, Log: slog.New(slog.NewTextHandler(log, nil)),
	})
}

func (r *Review) Answer(yes bool) {
	select {
	case r.answers <- yes:
	default:
	}
}

func (r *Review) confirm(ctx context.Context, _ string, commands []project.Command) (bool, error) {
	application.Get().Event.Emit(EventConfirm, commands)
	select {
	case yes := <-r.answers:
		return yes, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}
