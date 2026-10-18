package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/sandbox"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           sandbox.NewServer(log, filepath.Join(os.TempDir(), "eyeful-sandbox"), 10*time.Minute),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdown); err != nil {
			log.Error("shutdown", slog.Any("error", err))
		}
	}()
	log.Info("listening", slog.String("addr", srv.Addr))
	err := srv.ListenAndServe()
	stop()
	if !errors.Is(err, http.ErrServerClosed) {
		log.Error("serve", slog.Any("error", err))
		os.Exit(1)
	}
}
