package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Pleasurecruise/eyeful/internal/app"
	"github.com/Pleasurecruise/eyeful/internal/config"
	local "github.com/Pleasurecruise/eyeful/local/app"
)

type serveCmd struct {
	config.Config `embed:""`
}

func (s *serveCmd) Run() error {
	cfg := s.Config
	cfg.Version = version
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "EYEFUL_") {
			if err := os.Unsetenv(name); err != nil {
				return fmt.Errorf("clear %s: %w", name, err)
			}
		}
	}
	if _, err := local.Connect(context.Background(), cfg.Agent); err != nil {
		return fmt.Errorf("agent: %w", err)
	}
	application := app.New(cfg, log)
	if err := application.Err(); err != nil {
		return err
	}
	start, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := application.Start(start); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	sig := <-application.Done()
	log.Info("stopping", slog.String("signal", sig.String()))
	stop, cancelStop := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelStop()
	if err := application.Stop(stop); err != nil {
		return fmt.Errorf("stop: %w", err)
	}
	return nil
}
