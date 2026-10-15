package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/Pleasurecruise/eyeful/internal/config"
	"github.com/Pleasurecruise/eyeful/internal/db"
	"github.com/Pleasurecruise/eyeful/internal/handlers"
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
	fx.Provide(newPool),
)

var ReviewModule = fx.Module("review",
	fx.Provide(newReviewStore),
)

var SchedulerModule = fx.Module("scheduler",
	fx.Provide(newWorkerPool),
	fx.Invoke(runWorkers),
)

var HTTPModule = fx.Module("http",
	fx.Provide(newServer),
	fx.Invoke(runServer),
)

func newPool(lc fx.Lifecycle, cfg config.Config) (*pgxpool.Pool, error) {
	if !cfg.NoMigrate {
		migrator, err := db.NewMigrator(cfg.DatabaseURL)
		if err != nil {
			return nil, err
		}
		if err := errors.Join(migrator.Up(), migrator.Close()); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	lc.Append(fx.StopHook(pool.Close))
	return pool, nil
}

func newReviewStore(pool *pgxpool.Pool, cfg config.Config) *review.PostgresStore {
	return review.NewPostgresStore(pool, cfg.MaxAttempts)
}

func newWorkerPool(log *slog.Logger, cfg config.Config, store *review.PostgresStore) (*scheduler.WorkerPool, error) {
	host, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname for lease owner: %w", err)
	}
	// TODO(review): replace pending with workflow.Run; Outcome carries its result, with ResultPartial.
	// TODO(sandbox): Docker sandbox with snapshots, egress allowlist and orphan reconciliation.
	// TODO(runtime): pi adapter running agents inside the sandbox.
	// TODO(source): GitHub App webhook producer, with installation tokens minted per review.
	// TODO(sink): Markdown, JSON, GitHub review and Check Run consumers with an outbox.
	pending := scheduler.ExecutorFunc(func(context.Context, review.Review) error {
		return errors.New("the review pipeline is not implemented yet")
	})
	return scheduler.NewWorkerPool(log, store, pending, scheduler.Options{Owner: fmt.Sprintf("%s:%d", host, os.Getpid()), Workers: cfg.Workers, LeaseTTL: cfg.LeaseTTL, Poll: cfg.Poll}), nil
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

func newServer(log *slog.Logger, cfg config.Config, pool *pgxpool.Pool) *server.Server {
	return server.New(log, handlers.HealthRoutes{Version: cfg.Version, Ping: pool.Ping})
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
