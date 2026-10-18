package config

import (
	"log/slog"
	"time"
)

type Config struct {
	Addr            string        `default:":8080" env:"EYEFUL_ADDR" help:"Listen address."`
	ShutdownTimeout time.Duration `default:"30s" help:"How long stopping may take before it is cut short."`
	LogLevel        slog.Level    `default:"info" env:"EYEFUL_LOG_LEVEL" help:"debug, info, warn or error."`
	Review          string        `required:"" env:"EYEFUL_REVIEW" help:"The pull request to review, as JSON; the Worker sets it."`
	RunsURL         string        `required:"" env:"EYEFUL_RUNS_URL" help:"Where the review's stages are kept, so a restart resumes them; the Worker sets it."`
	AppID           int64         `required:"" env:"EYEFUL_GITHUB_APP_ID" help:"GitHub App id used to mint an installation token for the review."`
	PrivateKey      string        `required:"" env:"EYEFUL_GITHUB_APP_PRIVATE_KEY" help:"GitHub App private key. A PEM, or a path to a PEM file."`
	GitHubAPI       string        `default:"https://api.github.com" env:"EYEFUL_GITHUB_API_URL" help:"GitHub API root."`
	Agent           string        `default:"pi" env:"EYEFUL_AGENT" help:"Agent CLI the review runs: pi, claude or codex."`
	TypesafeKey     string        `env:"EYEFUL_TYPESAFE_API_KEY" help:"TypeSafe API key. Without it, findings on the same lines are not merged."`
	Version         string        `kong:"-"`
}
