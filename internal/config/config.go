package config

import (
	"log/slog"
)

type Config struct {
	LogLevel   slog.Level `default:"info" env:"EYEFUL_LOG_LEVEL" help:"debug, info, warn or error."`
	Repo       string     `required:"" env:"GITHUB_REPOSITORY" help:"The repository, as owner/name; Actions sets it."`
	Pull       int64      `required:"" env:"EYEFUL_PULL" help:"The pull request to review."`
	RunID      int64      `required:"" env:"GITHUB_RUN_ID" help:"The workflow run doing the review. It marks what the review posts, so a re-run posts nothing twice, and the review does not wait for it. Actions sets it."`
	Dir        string     `default:"/work" env:"EYEFUL_DIR" help:"Directory for the checkout and the review's files; the sandbox mounts it read-only at the same path."`
	SandboxURL string     `default:"http://eyeful-sandbox:8080" env:"EYEFUL_SANDBOX_URL" help:"Where the review's sandbox runs the project's commands."`
	Token      string     `required:"" env:"EYEFUL_GITHUB_TOKEN" help:"Token that clones, reads Actions and posts the review: the job's GITHUB_TOKEN."`
	GitHubAPI  string     `default:"https://api.github.com" env:"GITHUB_API_URL" help:"GitHub API root; Actions sets it."`
	ModelKey   string     `required:"" env:"EYEFUL_MODEL_KEY" help:"API key for that provider."`
	Jev        string     `required:"" env:"EYEFUL_TYPESAFE_API_KEY" help:"TypeSafe API key for Jev, which says which findings on the same lines are one problem."`
	Agent      string     `default:"pi" env:"EYEFUL_AGENT" help:"Agent CLI the review runs: pi, claude or codex."`
}
