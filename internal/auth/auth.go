package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Pleasurecruise/eyeful/internal/db/sqlc"
)

type Service struct {
	pool       *pgxpool.Pool
	q          *sqlc.Queries
	sessionTTL time.Duration
}

func NewService(pool *pgxpool.Pool, sessionTTL time.Duration) *Service {
	return &Service{pool: pool, q: sqlc.New(pool), sessionTTL: sessionTTL}
}

func (s *Service) DeleteSessionByToken(ctx context.Context, token string) error {
	return s.q.DeleteSession(ctx, hashToken(token))
}

func (s *Service) Authenticate(ctx context.Context, bearer, session string) (Principal, error) {
	if bearer != "" {
		if !strings.HasPrefix(bearer, apiKeyPrefix) {
			return Principal{}, ErrUnauthenticated
		}
		row, err := s.q.UseAPIKey(ctx, hashToken(bearer))
		if errors.Is(err, pgx.ErrNoRows) {
			return Principal{}, ErrUnauthenticated
		}
		if err != nil {
			return Principal{}, fmt.Errorf("use api key: %w", err)
		}
		user, err := s.q.GetUser(ctx, row.UserID)
		if err != nil {
			return Principal{}, fmt.Errorf("get user: %w", err)
		}
		return Principal{User: userOf(user), APIKeyID: row.ID}, nil
	}
	if session == "" {
		return Principal{}, ErrUnauthenticated
	}
	row, err := s.q.GetSessionUser(ctx, hashToken(session))
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("get session: %w", err)
	}
	return Principal{User: userOf(row.User), SessionID: row.SessionID}, nil
}

func (s *Service) CreateAPIKey(ctx context.Context, userID, name string) (APIKey, string, error) {
	secret := apiKeyPrefix + strings.ToLower(rand.Text())
	row, err := s.q.CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		ID:      newID("key_"),
		UserID:  userID,
		Name:    strings.TrimSpace(name),
		Prefix:  secret[:len(apiKeyPrefix)+6],
		KeyHash: hashToken(secret),
	})
	if err != nil {
		return APIKey{}, "", fmt.Errorf("create api key: %w", err)
	}
	return APIKey{ID: row.ID, Name: row.Name, Prefix: row.Prefix, CreatedAt: row.CreatedAt.Time}, secret, nil
}

func (s *Service) ListAPIKeys(ctx context.Context, userID string) ([]APIKey, error) {
	rows, err := s.q.ListAPIKeys(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	out := make([]APIKey, len(rows))
	for i, row := range rows {
		out[i] = APIKey{ID: row.ID, Name: row.Name, Prefix: row.Prefix, LastUsedAt: row.LastUsedAt.Time, CreatedAt: row.CreatedAt.Time}
	}
	return out, nil
}

func (s *Service) DeleteAPIKey(ctx context.Context, userID, id string) error {
	n, err := s.q.DeleteAPIKey(ctx, sqlc.DeleteAPIKeyParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if n == 0 {
		return ErrKeyNotFound
	}
	return nil
}

func (s *Service) issueSession(ctx context.Context, q *sqlc.Queries, userID string, c ClientInfo) (SessionToken, error) {
	token := strings.ToLower(rand.Text() + rand.Text())
	expires := time.Now().Add(s.sessionTTL)
	err := q.CreateSession(ctx, sqlc.CreateSessionParams{
		ID:        newID("ses_"),
		UserID:    userID,
		TokenHash: hashToken(token),
		ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true},
		Ip:        c.IP,
		UserAgent: c.UserAgent,
	})
	if err != nil {
		return SessionToken{}, fmt.Errorf("create session: %w", err)
	}
	return SessionToken{Token: token, ExpiresAt: expires}, nil
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func newID(prefix string) string {
	return prefix + strings.ToLower(rand.Text()[:16])
}

func userOf(row sqlc.User) User {
	return User{ID: row.ID, Email: row.Email, Name: row.Name, CreatedAt: row.CreatedAt.Time}
}

func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

func (s *Service) UpdateUser(ctx context.Context, userID, name string) (User, error) {
	row, err := s.q.UpdateUser(ctx, sqlc.UpdateUserParams{ID: userID, Name: strings.TrimSpace(name)})
	if err != nil {
		return User{}, fmt.Errorf("update user: %w", err)
	}
	return userOf(row), nil
}

func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	if _, err := s.q.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (s *Service) ListSessions(ctx context.Context, userID string) ([]Session, error) {
	rows, err := s.q.ListSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	out := make([]Session, len(rows))
	for i, row := range rows {
		out[i] = Session{ID: row.ID, IP: row.Ip, UserAgent: row.UserAgent, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
	}
	return out, nil
}

func (s *Service) DeleteSession(ctx context.Context, userID, id string) error {
	n, err := s.q.DeleteSessionByID(ctx, sqlc.DeleteSessionByIDParams{ID: id, UserID: userID})
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}

func (s *Service) GetAPIKey(ctx context.Context, userID, id string) (APIKey, error) {
	row, err := s.q.GetAPIKey(ctx, sqlc.GetAPIKeyParams{ID: id, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, ErrKeyNotFound
	}
	if err != nil {
		return APIKey{}, fmt.Errorf("get api key: %w", err)
	}
	return APIKey{ID: row.ID, Name: row.Name, Prefix: row.Prefix, LastUsedAt: row.LastUsedAt.Time, CreatedAt: row.CreatedAt.Time}, nil
}

func (s *Service) UpdateAPIKey(ctx context.Context, userID, id, name string) (APIKey, error) {
	row, err := s.q.UpdateAPIKey(ctx, sqlc.UpdateAPIKeyParams{ID: id, UserID: userID, Name: strings.TrimSpace(name)})
	if errors.Is(err, pgx.ErrNoRows) {
		return APIKey{}, ErrKeyNotFound
	}
	if err != nil {
		return APIKey{}, fmt.Errorf("update api key: %w", err)
	}
	return APIKey{ID: row.ID, Name: row.Name, Prefix: row.Prefix, LastUsedAt: row.LastUsedAt.Time, CreatedAt: row.CreatedAt.Time}, nil
}
