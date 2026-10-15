package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

type WorkerPool struct {
	q    Queue
	exec Executor
	opts Options
	log  *slog.Logger
}

func (f ExecutorFunc) Execute(ctx context.Context, r review.Review) error { return f(ctx, r) }

func NewWorkerPool(log *slog.Logger, q Queue, exec Executor, opts Options) *WorkerPool {
	return &WorkerPool{q: q, exec: exec, opts: opts, log: log.With(slog.String("component", "scheduler"))}
}

func (p *WorkerPool) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for range p.opts.Workers {
		wg.Go(func() { p.work(ctx) })
	}
	wg.Go(func() { p.reap(ctx) })
	wg.Wait()
}

func (p *WorkerPool) work(ctx context.Context) {
	for ctx.Err() == nil {
		r, l, err := p.q.Claim(ctx, p.opts.Owner, p.opts.LeaseTTL)
		if errors.Is(err, review.ErrQueueEmpty) {
			sleep(ctx, p.opts.Poll)
			continue
		}
		if err != nil && ctx.Err() != nil {
			return
		}
		if err != nil {
			p.log.Error("claim", slog.String("error", err.Error()))
			sleep(ctx, p.opts.Poll)
			continue
		}
		p.run(ctx, r, l)
	}
}

func (p *WorkerPool) run(ctx context.Context, r review.Review, l review.Lease) {
	log := p.log.With(slog.String("review", r.ID), slog.Int64("token", l.FencingToken))
	log.Info("start", slog.Int64("attempt", int64(r.Attempts)))
	execCtx, stop := context.WithCancelCause(ctx)
	defer stop(nil)

	heartbeat := make(chan struct{})
	go func() {
		defer close(heartbeat)
		p.renew(execCtx, l, stop)
	}()
	err := p.exec.Execute(execCtx, r)
	stop(nil)
	<-heartbeat

	bg := context.WithoutCancel(ctx)
	var outcome review.Outcome
	switch cause := context.Cause(execCtx); {
	case errors.Is(cause, errFenced):
		log.Warn("lease lost; result dropped")
		return
	case ctx.Err() != nil:
		if rerr := p.q.Release(bg, l); rerr != nil {
			log.Error("release", slog.String("error", rerr.Error()))
		}
		log.Info("released on shutdown")
		return
	case err != nil:
		outcome = review.Outcome{Result: review.ResultNone, Reason: err.Error()}
	default:
		outcome = review.Outcome{Result: review.ResultComplete}
	}
	if ferr := p.q.Finish(bg, l, outcome); ferr != nil {
		log.Error("finish", slog.String("error", ferr.Error()))
		return
	}
	log.Info("finish", slog.String("result", string(outcome.Result)))
}

func (p *WorkerPool) renew(ctx context.Context, l review.Lease, stop context.CancelCauseFunc) {
	t := time.NewTicker(p.opts.LeaseTTL / 3)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		switch err := p.q.Renew(ctx, l, p.opts.LeaseTTL); {
		case err == nil:
		case errors.Is(err, review.ErrFenced), errors.Is(err, review.ErrNotFound):
			stop(errFenced)
			return
		default:
			p.log.Warn("renew", slog.String("review", l.ReviewID), slog.String("error", err.Error()))
		}
	}
}

func (p *WorkerPool) reap(ctx context.Context) {
	t := time.NewTicker(p.opts.LeaseTTL)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		if n, err := p.q.Reap(ctx); err != nil {
			p.log.Error("reap", slog.String("error", err.Error()))
		} else if n > 0 {
			p.log.Warn("reaped expired leases", slog.Int("count", n))
		}
	}
}

func sleep(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
