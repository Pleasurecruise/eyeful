package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v5"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/Pleasurecruise/eyeful/apps/web"
	"github.com/Pleasurecruise/eyeful/internal/auth"
	"github.com/Pleasurecruise/eyeful/internal/config"
	"github.com/Pleasurecruise/eyeful/internal/db"
	"github.com/Pleasurecruise/eyeful/internal/handlers"
	"github.com/Pleasurecruise/eyeful/internal/repository"
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
		AuthModule,
		ReviewModule,
		RepositoryModule,
		SchedulerModule,
		HTTPModule,
	)
}

var DatabaseModule = fx.Module("database",
	fx.Provide(newPool),
)

var AuthModule = fx.Module("auth",
	fx.Provide(newAuthService, newSealer, newProviders),
)

var ReviewModule = fx.Module("review",
	fx.Provide(newReviewStore),
)

var RepositoryModule = fx.Module("repository",
	fx.Provide(newRepositoryService),
)

var SchedulerModule = fx.Module("scheduler",
	fx.Provide(newWorkerPool),
	fx.Invoke(runWorkers),
)

// TODO(mcp): POST /mcp via go-sdk StreamableHTTPHandler{Stateless: true}, protocol versions 2025-11-25 and 2026-07-28, tools mirroring the review CRUD.
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

func newAuthService(pool *pgxpool.Pool, cfg config.Config) *auth.Service {
	return auth.NewService(pool, cfg.SessionTTL)
}

func newReviewStore(pool *pgxpool.Pool, cfg config.Config) *review.PostgresStore {
	return review.NewPostgresStore(pool, cfg.MaxAttempts)
}

func newSealer(cfg config.Config) (*auth.Sealer, error) {
	key, err := base64.StdEncoding.DecodeString(cfg.TokenKey)
	if err != nil {
		return nil, fmt.Errorf("EYEFUL_TOKEN_KEY: %w", err)
	}
	return auth.NewSealer(key)
}

type providers struct {
	github *auth.OAuth
	all    []handlers.OAuthProvider
}

func newProviders(cfg config.Config, accounts *auth.Service, sealer *auth.Sealer) providers {
	callback := func(p auth.Provider) string {
		return strings.TrimSuffix(cfg.PublicURL, "/") + "/oauth/" + p.ID + "/callback"
	}
	gh := auth.NewOAuth(accounts, sealer, auth.GitHub, auth.Client{ID: cfg.GitHubClientID, Secret: cfg.GitHubClientSecret, RedirectURL: callback(auth.GitHub)})
	all := []handlers.OAuthProvider{gh}
	if cfg.GiteeClientID != "" {
		all = append(all, auth.NewOAuth(accounts, sealer, auth.Gitee, auth.Client{ID: cfg.GiteeClientID, Secret: cfg.GiteeClientSecret, RedirectURL: callback(auth.Gitee)}))
	}
	return providers{github: gh, all: all}
}

func newRepositoryService(p providers) *repository.Service {
	return repository.NewService(p.github, auth.GitHub.APIBase+"/")
}

func newWorkerPool(log *slog.Logger, cfg config.Config, store *review.PostgresStore) (*scheduler.WorkerPool, error) {
	host, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname for lease owner: %w", err)
	}
	// TODO(review): replace pending with workflow.Run; Outcome carries its result, with ResultPartial.
	// TODO(sandbox): Docker sandbox with snapshots, egress allowlist and orphan reconciliation.
	// TODO(runtime): pi adapter running agents inside the sandbox.
	// TODO(source): GitHub PR, webhook and scheduled producers.
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

func newServer(log *slog.Logger, cfg config.Config, pool *pgxpool.Pool, accounts *auth.Service, oauth providers, reviews *review.PostgresStore, repos *repository.Service) (*server.Server, error) {
	public, err := url.Parse(cfg.PublicURL)
	if err != nil {
		return nil, fmt.Errorf("public url: %w", err)
	}
	opts := server.Options{
		Authenticate:   accounts.Authenticate,
		PublicRoutes:   handlers.PublicRoutes,
		RateLimit:      cfg.RateLimit,
		TrustedOrigins: append([]string{public.Scheme + "://" + public.Host}, cfg.CORSOrigins...),
		CORSOrigins:    cfg.CORSOrigins,
	}
	if web.Built() {
		dist, err := fs.Sub(web.Assets, "dist")
		if err != nil {
			return nil, fmt.Errorf("console assets: %w", err)
		}
		opts.Assets = http.FS(dist)
	}
	secure := !cfg.InsecureCookies
	critical := []echo.MiddlewareFunc{server.RateLimit("critical", cfg.CriticalRateLimit, time.Minute)}
	routes := []server.Handler{
		handlers.HealthRoutes{Version: cfg.Version, Ping: pool.Ping},
		handlers.DocsRoutes{},
		handlers.UserRoutes{Users: accounts, SecureCookies: secure},
		handlers.SessionRoutes{Sessions: accounts, SecureCookies: secure},
		handlers.APIKeyRoutes{Store: accounts},
		handlers.ReviewRoutes{Store: reviews, CreateRateLimits: critical},
		handlers.RepositoryRoutes{Repos: repos},
		handlers.OAuthRoutes{Providers: oauth.all, SecureCookies: secure, RateLimits: critical},
	}
	return server.New(log, opts, routes...), nil
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
