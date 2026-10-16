package config

import (
	"errors"
	"log/slog"
	"time"
)

type Config struct {
	Addr            string        `default:":8080" env:"EYEFUL_ADDR" help:"Listen address."`
	D1AccountID     string        `required:"" env:"EYEFUL_D1_ACCOUNT_ID" help:"Account segment of the D1 query path; the Worker sets it."`
	D1DatabaseID    string        `required:"" env:"EYEFUL_D1_DATABASE_ID" help:"Database segment of the D1 query path; the Worker sets it."`
	D1APIToken      string        `required:"" env:"EYEFUL_D1_API_TOKEN" help:"Bearer token for the D1 query endpoint; the Worker sets it."`
	D1API           string        `required:"" env:"EYEFUL_D1_API" help:"Root of the D1 query endpoint; the Worker sets it."`
	Workers         int           `default:"2" help:"Reviews this process runs at once."`
	LeaseTTL        time.Duration `default:"30s" name:"lease-ttl" help:"How long a worker's claim on a review lasts without a heartbeat."`
	Poll            time.Duration `default:"1s" help:"How long an idle worker waits before it looks for a queued review again."`
	MaxAttempts     int32         `default:"3" help:"Lost workers a review survives before it fails."`
	ShutdownTimeout time.Duration `default:"30s" help:"How long stopping may take before it is cut short."`
	LogLevel        slog.Level    `default:"info" env:"EYEFUL_LOG_LEVEL" help:"debug, info, warn or error."`
	WebhookSecret   string        `env:"EYEFUL_WEBHOOK_SECRET" help:"GitHub webhook secret checked against X-Hub-Signature-256."`
	AppID           int64         `env:"EYEFUL_GITHUB_APP_ID" help:"GitHub App id used to mint an installation token per review."`
	PrivateKey      string        `env:"EYEFUL_GITHUB_APP_PRIVATE_KEY" help:"GitHub App private key. A PEM, or a path to a PEM file."`
	GitHubAPI       string        `default:"https://api.github.com" env:"EYEFUL_GITHUB_API_URL" help:"GitHub API root."`
	Agent           string        `default:"pi" env:"EYEFUL_AGENT" help:"Agent CLI the cloud harness runs: pi, claude or codex."`
	DataDir         string        `env:"EYEFUL_DATA_DIR" help:"Directory for a review's stage archive. Empty uses a directory under the system temp directory."`
	TypesafeKey     string        `env:"EYEFUL_TYPESAFE_API_KEY" help:"TypeSafe API key. Without it, findings on the same lines are not merged."`
	Version         string        `kong:"-"`
}

var ErrInvalidWorkers = errors.New("workers, lease TTL and poll interval must be positive")

func (c Config) Validate() error {
	if c.Workers < 1 || c.LeaseTTL <= 0 || c.Poll <= 0 {
		return ErrInvalidWorkers
	}
	return nil
}
