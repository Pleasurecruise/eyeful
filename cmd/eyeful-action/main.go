package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kong"

	"github.com/Pleasurecruise/eyeful/internal/checkout"
	"github.com/Pleasurecruise/eyeful/internal/config"
	"github.com/Pleasurecruise/eyeful/internal/github"
	"github.com/Pleasurecruise/eyeful/internal/harness"
	"github.com/Pleasurecruise/eyeful/internal/models"
	"github.com/Pleasurecruise/eyeful/internal/sandbox"
	local "github.com/Pleasurecruise/eyeful/local/app"
	"github.com/Pleasurecruise/eyeful/local/classify"
)

var version = "dev"

const (
	modelsAddr = "127.0.0.1:8081"
	modelsURL  = "https://ark.cn-beijing.volces.com/api/coding/v3"
)

type cli struct {
	Version kong.VersionFlag `short:"v" help:"Print the version and exit."`

	config.Config `embed:""`
}

func main() {
	var c cli
	kong.Parse(&c,
		kong.Name("eyeful-action"),
		kong.Description("Review one pull request in a GitHub Actions job and post the result."),
		kong.Vars{"version": version},
		kong.ConfigureHelp(kong.HelpOptions{Compact: true}),
	)
	if err := c.run(); err != nil {
		fmt.Fprintln(os.Stderr, "eyeful-action:", err)
		os.Exit(1)
	}
}

func (c *cli) run() error {
	cfg := c.Config
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "EYEFUL_") {
			if err := os.Unsetenv(name); err != nil {
				return fmt.Errorf("clear %s: %w", name, err)
			}
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if _, err := local.Connect(ctx, cfg.Agent); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	upstream, err := url.Parse(modelsURL)
	if err != nil {
		return fmt.Errorf("model provider: %w", err)
	}
	var listen net.ListenConfig
	ln, err := listen.Listen(ctx, "tcp", modelsAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", modelsAddr, err)
	}
	srv := &http.Server{Handler: models.Proxy(upstream, cfg.ModelKey), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
			log.Error("models proxy", slog.Any("error", err))
		}
	}()
	defer func() { _ = srv.Close() }()
	gh := github.New(cfg.GitHubAPI, cfg.Token, cfg.RunID)
	src, open, err := gh.Pull(ctx, cfg.Repo, cfg.Pull)
	if err != nil {
		return fmt.Errorf("pull request: %w", err)
	}
	if !open {
		log.Info("skipped", slog.String("repo", src.Repo), slog.Int64("pull", src.Number), slog.String("reason", "the pull request is closed"))
		return nil
	}
	jev, err := classify.New(log, cfg.Jev)
	if err != nil {
		return err
	}
	runner := harness.Runner{
		ID: strconv.FormatInt(cfg.RunID, 10), Log: log, Clone: checkout.Clone, Dir: cfg.Dir, Token: cfg.Token,
		Classifier: jev, Sandbox: sandbox.NewClient(cfg.SandboxURL), Host: gh,
	}
	status, err := runner.Execute(ctx, src)
	if err != nil {
		return err
	}
	log.Info("done", slog.String("repo", src.Repo), slog.Int64("pull", src.Number), slog.String("outcome", string(status.Outcome)), slog.String("reason", status.Reason))
	return nil
}
