package config

import (
	"errors"
	"log/slog"
	"time"
)

type Config struct {
	Addr            string        `default:":8080" env:"EYEFUL_ADDR" help:"Listen address."`
	DatabaseURL     string        `required:"" env:"EYEFUL_DATABASE_URL" help:"PostgreSQL connection URL."`
	NoMigrate       bool          `help:"Do not apply pending migrations at start-up."`
	Workers         int           `default:"2" help:"Reviews this process runs at once."`
	LeaseTTL        time.Duration `default:"30s" name:"lease-ttl" help:"How long a worker's claim on a review lasts without a heartbeat."`
	Poll            time.Duration `default:"1s" help:"How long an idle worker waits before it looks for a queued review again."`
	MaxAttempts     int32         `default:"3" help:"Lost workers a review survives before it fails."`
	ShutdownTimeout time.Duration `default:"30s" help:"How long stopping may take before it is cut short."`
	LogLevel        slog.Level    `default:"info" env:"EYEFUL_LOG_LEVEL" help:"debug, info, warn or error."`
	Version         string        `kong:"-"`
}

var ErrInvalidWorkers = errors.New("workers, lease TTL and poll interval must be positive")

func (c Config) Validate() error {
	if c.Workers < 1 || c.LeaseTTL <= 0 || c.Poll <= 0 {
		return ErrInvalidWorkers
	}
	return nil
}
