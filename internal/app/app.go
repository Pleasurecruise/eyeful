package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/Pleasurecruise/eyeful/internal/checkout"
	"github.com/Pleasurecruise/eyeful/internal/config"
	"github.com/Pleasurecruise/eyeful/internal/db"
	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/handlers"
	"github.com/Pleasurecruise/eyeful/internal/harness"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/scheduler"
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
		DatabaseModule,
		ReviewModule,
		SchedulerModule,
		HTTPModule,
	)
}

var DatabaseModule = fx.Module("database",
	fx.Provide(newDB),
)

var ReviewModule = fx.Module("review",
	fx.Provide(newReviewStore),
)

var SchedulerModule = fx.Module("scheduler",
	fx.Provide(newGitHub, newHarness, newWorkerPool),
	fx.Invoke(runWorkers),
)

var HTTPModule = fx.Module("http",
	fx.Provide(newServer),
	fx.Invoke(runServer),
)

func newDB(cfg config.Config) (*db.Client, error) {
	client, err := db.New(cfg.D1API, cfg.D1AccountID, cfg.D1DatabaseID, cfg.D1APIToken)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

func newReviewStore(client *db.Client, cfg config.Config) *review.Store {
	return review.New(client, cfg.MaxAttempts)
}

func newGitHub(cfg config.Config) (*github.App, error) {
	key, err := github.ParseKey(cfg.PrivateKey)
	if err != nil && !errors.Is(err, github.ErrNoKey) {
		return nil, err
	}
	return github.New(cfg.GitHubAPI, cfg.AppID, key), nil
}

func newHarness(log *slog.Logger, cfg config.Config, gh *github.App) *harness.Runner {
	return &harness.Runner{Log: log, Clone: checkout.Clone, DataDir: cfg.DataDir, Agent: cfg.Agent, TypesafeKey: cfg.TypesafeKey, Host: gh}
}

func newWorkerPool(log *slog.Logger, cfg config.Config, store *review.Store, runner *harness.Runner) (*scheduler.WorkerPool, error) {
	host, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname for lease owner: %w", err)
	}
	// TODO(sandbox): Docker sandbox with snapshots, egress allowlist and orphan reconciliation.
	// TODO(sink): an outbox, so a crash after GitHub accepts a comment cannot post it twice.
	return scheduler.NewWorkerPool(log, store, runner, scheduler.Options{Owner: fmt.Sprintf("%s:%d", host, os.Getpid()), Workers: cfg.Workers, LeaseTTL: cfg.LeaseTTL, Poll: cfg.Poll}), nil
}

func runWorkers(lc fx.Lifecycle, workers *scheduler.WorkerPool) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			go func() {
				defer close(done)
				workers.Run(ctx)
			}()
			return nil
		},
		// TODO(sandbox): end sandbox and agent processes before the workers release their reviews.
		OnStop: func(stop context.Context) error {
			cancel()
			select {
			case <-done:
				return nil
			case <-stop.Done():
				return fmt.Errorf("workers did not drain: %w", stop.Err())
			}
		},
	})
}

func newServer(log *slog.Logger, cfg config.Config, client *db.Client, store *review.Store, gh *github.App) *server.Server {
	return server.New(log,
		handlers.HealthRoutes{Version: cfg.Version, Ping: client.Ping},
		handlers.Webhook{Secret: cfg.WebhookSecret, Configured: gh.Key != nil && gh.AppID >= 1, Admit: store},
	)
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
