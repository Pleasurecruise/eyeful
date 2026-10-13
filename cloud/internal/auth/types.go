package auth

import (
	"crypto/cipher"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

const (
	SessionCookie = "eyeful_session"
	apiKeyPrefix  = "eyf_"
)

var (
	ErrUnauthenticated = errors.New("not signed in")
	ErrKeyNotFound     = errors.New("api key not found")
	ErrSessionNotFound = errors.New("session not found")
	ErrNoVerifiedEmail = errors.New("account has no verified primary email")
	ErrNotLinked       = errors.New("no account with this provider")
	ErrGrantExpired    = errors.New("provider grant expired; sign in again")
	ErrTokenKeySize    = errors.New("must be 32 bytes")
	ErrSealedToken     = errors.New("sealed token cannot be opened")
)

type grant struct {
	Scopes           []string
	AccessToken      []byte
	AccessExpiresAt  pgtype.Timestamptz
	RefreshToken     []byte
	RefreshExpiresAt pgtype.Timestamptz
}

type Sealer struct {
	aead cipher.AEAD
}

type User struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

type Principal struct {
	User      User
	SessionID string
	APIKeyID  string
}

type SessionToken struct {
	Token     string
	ExpiresAt time.Time
}

type ClientInfo struct {
	IP        string
	UserAgent string
}

type APIKey struct {
	ID         string
	Name       string
	Prefix     string
	LastUsedAt time.Time
	CreatedAt  time.Time
}

type principalKey struct{}

type Session struct {
	ID        string
	IP        string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Provider struct {
	ID       string
	Title    string
	Endpoint oauth2.Endpoint
	APIBase  string
	Scopes   []string
}

var (
	GitHub = Provider{
		ID:       "github",
		Title:    "GitHub",
		Endpoint: endpoints.GitHub,
		APIBase:  "https://api.github.com",
	}
	Gitee = Provider{
		ID:    "gitee",
		Title: "Gitee",
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://gitee.com/oauth/authorize",
			TokenURL:  "https://gitee.com/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		APIBase: "https://gitee.com/api/v5",
		Scopes:  []string{"user_info", "emails"},
	}
)

type Client struct {
	ID          string
	Secret      string
	RedirectURL string
}

type identity struct {
	ID    string
	Email string
	Name  string
}

type giteeUserDetail struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
}

type giteeUserEmail struct {
	Email string   `json:"email"`
	State string   `json:"state"`
	Scope []string `json:"scope"`
}
