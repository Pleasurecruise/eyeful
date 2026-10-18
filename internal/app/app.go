package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/Pleasurecruise/eyeful/internal/checkout"
	"github.com/Pleasurecruise/eyeful/internal/config"
	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/handlers"
	"github.com/Pleasurecruise/eyeful/internal/harness"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/runs"
	"github.com/Pleasurecruise/eyeful/internal/server"
)

func New(cfg config.Config, log *slog.Logger) *fx.App {
	return fx.New(
		fx.Supply(cfg, log),
		fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
			l := &fxevent.SlogLogger{Logger: log}
			l.UseLogLevel(slog.LevelDebug)
			return l
		}),
		fx.StartTimeout(cfg.ShutdownTimeout),
		fx.StopTimeout(cfg.ShutdownTimeout),
		ReviewModule,
		HTTPModule,
	)
}

var ReviewModule = fx.Module("review",
	fx.Provide(newSource, newGitHub, newHarness, newStatus),
	fx.Invoke(runReview),
)

var HTTPModule = fx.Module("http",
	fx.Provide(newServer),
	fx.Invoke(runServer),
)

func newSource(cfg config.Config) (review.Source, error) {
	var src review.Source
	dec := json.NewDecoder(bytes.NewReader([]byte(cfg.Review)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&src); err != nil {
		return src, fmt.Errorf("decode EYEFUL_REVIEW: %w", err)
	}
	return src, nil
}

func newGitHub(cfg config.Config) (*github.App, error) {
	key, err := github.ParseKey(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	return github.New(cfg.GitHubAPI, cfg.AppID, key), nil
}

func newHarness(log *slog.Logger, cfg config.Config, gh *github.App) harness.Runner {
	// TODO(sandbox): run the project's commands in a sandbox per review, then verify by execution.
	// TODO(sink): an outbox, so a crash after GitHub accepts a comment cannot post it twice.
	return harness.Runner{
		Log: log, Clone: checkout.Clone, TypesafeKey: cfg.TypesafeKey,
		Progress: os.Stderr, Runs: runs.New(cfg.RunsURL), Host: gh,
	}
}

func newStatus() *atomic.Pointer[harness.Status] {
	var status atomic.Pointer[harness.Status]
	status.Store(&harness.Status{State: harness.StateRunning})
	return &status
}

func runReview(lc fx.Lifecycle, log *slog.Logger, cfg config.Config, src review.Source, runner harness.Runner, status *atomic.Pointer[harness.Status]) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				result, err := runner.Execute(ctx, src)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Error("review failed", slog.String("repo", src.Repo), slog.Int64("pull", src.Number), slog.Any("error", err))
					result = harness.Status{State: harness.StateFailed, Reason: err.Error()}
				}
				status.Store(&result)
			}()
			return nil
		},
		OnStop: func(stop context.Context) error {
			cancel()
			select {
			case <-done:
				return nil
			case <-stop.Done():
				return fmt.Errorf("the review did not stop: %w", stop.Err())
			}
		},
	})
}

func newServer(log *slog.Logger, status *atomic.Pointer[harness.Status]) *server.Server {
	return server.New(log, handlers.ReviewRoutes{Status: func() harness.Status { return *status.Load() }})
}

func runServer(lc fx.Lifecycle, cfg config.Config, srv *server.Server) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	lc.Append(fx.Hook{
		OnStart: func(start context.Context) error {
			var listen net.ListenConfig
			ln, err := listen.Listen(start, "tcp", cfg.Addr)
			if err != nil {
				return fmt.Errorf("listen %s: %w", cfg.Addr, err)
			}
			go func() { done <- srv.Serve(ctx, ln) }()
			return nil
		},
		OnStop: func(stop context.Context) error {
			cancel()
			select {
			case err := <-done:
				return err
			case <-stop.Done():
				return fmt.Errorf("http server did not drain: %w", stop.Err())
			}
		},
	})
}
