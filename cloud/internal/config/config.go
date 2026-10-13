package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"
)

type Config struct {
	Addr               string        `default:":8080" env:"EYEFUL_ADDR" help:"Listen address."`
	DatabaseURL        string        `required:"" env:"EYEFUL_DATABASE_URL" help:"PostgreSQL connection URL."`
	PublicURL          string        `default:"http://localhost:8080" env:"EYEFUL_PUBLIC_URL" help:"URL browsers reach the console at; used for OAuth redirects and as the trusted origin."`
	CORSOrigins        []string      `name:"cors-origins" env:"EYEFUL_CORS_ORIGINS" sep:"," help:"Other origins, comma-separated, that may call the API with cookies, e.g. https://console.example.com."`
	NoMigrate          bool          `help:"Do not apply pending migrations at start-up."`
	InsecureCookies    bool          `name:"insecure-cookies" env:"EYEFUL_INSECURE_COOKIES" help:"Send session cookies without Secure so plain http works (local development only)."`
	SessionTTL         time.Duration `default:"720h" help:"How long a browser session lasts."`
	TokenKey           string        `required:"" env:"EYEFUL_TOKEN_KEY" help:"Base64 of 32 random bytes that encrypt stored provider tokens (openssl rand -base64 32)."`
	GitHubClientID     string        `required:"" name:"github-client-id" env:"EYEFUL_GITHUB_CLIENT_ID" help:"GitHub App client ID, for sign-in and repository access."`
	GitHubClientSecret string        `required:"" name:"github-client-secret" env:"EYEFUL_GITHUB_CLIENT_SECRET" help:"GitHub App client secret."`
	GiteeClientID      string        `name:"gitee-client-id" env:"EYEFUL_GITEE_CLIENT_ID" help:"Gitee OAuth app client ID; enables Gitee sign-in."`
	GiteeClientSecret  string        `name:"gitee-client-secret" env:"EYEFUL_GITEE_CLIENT_SECRET" help:"Gitee OAuth app client secret."`
	Workers            int           `default:"2" help:"Reviews this process runs at once."`
	LeaseTTL           time.Duration `default:"30s" name:"lease-ttl" help:"How long a worker's claim on a review lasts without a heartbeat."`
	Poll               time.Duration `default:"1s" help:"How long an idle worker waits before it looks for a queued review again."`
	MaxAttempts        int32         `default:"3" help:"Lost workers a review survives before it fails."`
	RateLimit          int           `default:"600" env:"EYEFUL_RATE_LIMIT" help:"API requests per minute per client; 0 disables."`
	CriticalRateLimit  int           `default:"30" env:"EYEFUL_CRITICAL_RATE_LIMIT" help:"Sign-ins and new reviews per minute per client; 0 disables."`
	ShutdownTimeout    time.Duration `default:"30s" help:"How long stopping may take before it is cut short."`
	LogLevel           slog.Level    `default:"info" env:"EYEFUL_LOG_LEVEL" help:"debug, info, warn or error."`
	Version            string        `kong:"-"`
}

var (
	ErrPartialProvider = errors.New("set both the client ID and the client secret")
	ErrInvalidWorkers  = errors.New("workers, lease TTL and poll interval must be positive")
	ErrInvalidOrigin   = errors.New("an origin is a scheme, a host and an optional port, such as https://console.example.com")
)

func (c Config) Validate() error {
	if c.Workers < 1 || c.LeaseTTL <= 0 || c.Poll <= 0 {
		return ErrInvalidWorkers
	}
	if (c.GiteeClientID == "") != (c.GiteeClientSecret == "") {
		return fmt.Errorf("gitee sign-in: %w", ErrPartialProvider)
	}
	for _, origin := range c.CORSOrigins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("cors origin %q: %w", origin, ErrInvalidOrigin)
		}
	}
	return nil
}
