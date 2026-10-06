package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/auth"
	"github.com/Pleasurecruise/eyeful/internal/db/dbtest"
)

var ctx = context.Background()

func service(t *testing.T) *auth.Service {
	t.Helper()
	return auth.NewService(dbtest.Open(t), time.Hour)
}

func TestSessions(t *testing.T) {
	s := service(t)
	user, first := signIn(t, s)
	_, second := signIn(t, s)
	p, err := s.Authenticate(t.Context(), "", second.Token)
	if err != nil || p.User.ID != user.ID || p.SessionID == "" {
		t.Fatalf("authenticate: %+v %v", p, err)
	}
	if err := s.DeleteSessionByToken(ctx, second.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), "", second.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("after sign-out: %v", err)
	}
	if _, err := s.Authenticate(t.Context(), "", first.Token); err != nil {
		t.Fatalf("other session must survive: %v", err)
	}
}

func TestAPIKeys(t *testing.T) {
	s := service(t)
	user, _ := signIn(t, s)
	key, secret, err := s.CreateAPIKey(ctx, user.ID, "ci")
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.Authenticate(t.Context(), secret, "")
	if err != nil || p.User.ID != user.ID || p.APIKeyID != key.ID {
		t.Fatalf("authenticate with key: %+v %v", p, err)
	}
	keys, _ := s.ListAPIKeys(ctx, user.ID)
	if len(keys) != 1 || keys[0].LastUsedAt.IsZero() || keys[0].Prefix != secret[:len(keys[0].Prefix)] {
		t.Fatalf("list: %+v", keys)
	}
	if err := s.DeleteAPIKey(ctx, "usr_other", key.ID); !errors.Is(err, auth.ErrKeyNotFound) {
		t.Fatalf("delete someone else's key: %v", err)
	}
	if err := s.DeleteAPIKey(ctx, user.ID, key.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(t.Context(), secret, ""); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("revoked key: %v", err)
	}
	for _, bad := range []string{"", "eyf_wrong", "not-a-key"} {
		if _, err := s.Authenticate(t.Context(), bad, ""); !errors.Is(err, auth.ErrUnauthenticated) {
			t.Errorf("bearer %q: %v", bad, err)
		}
	}
}

func TestSessionExpiry(t *testing.T) {
	s := auth.NewService(dbtest.Open(t), -time.Second)
	_, session := signIn(t, s)
	if _, err := s.Authenticate(t.Context(), "", session.Token); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("expired session: %v", err)
	}
}
