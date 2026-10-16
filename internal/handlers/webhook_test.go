package handlers_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/internal/handlers"
	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/internal/review/reviewtest"
	"github.com/Pleasurecruise/eyeful/internal/server"
)

const (
	headSHA = "0123456789abcdef0123456789abcdef01234567"
	baseSHA = "fedcba9876543210fedcba9876543210fedcba98"
	secret  = "webhook-secret"
)

func TestWebhook(t *testing.T) {
	t.Run("replay", func(t *testing.T) {
		admit := &counting{MemoryStore: reviewtest.NewMemoryStore(3)}
		srv := server.New(discard(), handlers.Webhook{Secret: secret, Configured: true, Admit: admit})
		first := post(srv, "pull_request", pull(false), secret)
		second := post(srv, "pull_request", pull(false), secret)
		if first.Code != http.StatusOK || second.Code != http.StatusOK || admit.calls != 2 {
			t.Fatalf("%d %d calls %d", first.Code, second.Code, admit.calls)
		}
		a, b := queued{}, queued{}
		must(t, json.Unmarshal(first.Body.Bytes(), &a))
		must(t, json.Unmarshal(second.Body.Bytes(), &b))
		if !a.Created || b.Created || a.ReviewID == "" || a.ReviewID != b.ReviewID {
			t.Fatalf("%+v %+v", a, b)
		}
	})
	t.Run("bad signature never queues", func(t *testing.T) {
		admit := &counting{MemoryStore: reviewtest.NewMemoryStore(3)}
		srv := server.New(discard(), handlers.Webhook{Secret: secret, Configured: true, Admit: admit})
		res := post(srv, "pull_request", pull(false), "other")
		if res.Code != http.StatusUnauthorized || admit.calls != 0 || problemCode(t, res) != "unauthorized" {
			t.Fatalf("%d %s calls %d", res.Code, problemCode(t, res), admit.calls)
		}
	})
	t.Run("missing secret", func(t *testing.T) {
		admit := &counting{MemoryStore: reviewtest.NewMemoryStore(3)}
		srv := server.New(discard(), handlers.Webhook{Configured: true, Admit: admit})
		res := post(srv, "pull_request", pull(false), secret)
		if res.Code != http.StatusInternalServerError || admit.calls != 0 {
			t.Fatalf("%d calls %d", res.Code, admit.calls)
		}
	})
	t.Run("app not configured", func(t *testing.T) {
		admit := &counting{MemoryStore: reviewtest.NewMemoryStore(3)}
		srv := server.New(discard(), handlers.Webhook{Secret: secret, Admit: admit})
		res := post(srv, "pull_request", pull(false), secret)
		if res.Code != http.StatusInternalServerError || admit.calls != 0 {
			t.Fatalf("%d calls %d", res.Code, admit.calls)
		}
	})
	t.Run("conflict", func(t *testing.T) {
		admit := &counting{MemoryStore: reviewtest.NewMemoryStore(3)}
		srv := server.New(discard(), handlers.Webhook{Secret: secret, Configured: true, Admit: admit})
		if post(srv, "pull_request", pull(false), secret).Code != http.StatusOK {
			t.Fatal("first")
		}
		res := post(srv, "pull_request", strings.Replace(pull(false), baseSHA, strings.Repeat("c", 40), 1), secret)
		if res.Code != http.StatusConflict || problemCode(t, res) != "conflict" {
			t.Fatalf("%d %s", res.Code, problemCode(t, res))
		}
	})
	t.Run("ignored", func(t *testing.T) {
		admit := &counting{MemoryStore: reviewtest.NewMemoryStore(3)}
		srv := server.New(discard(), handlers.Webhook{Secret: secret, Configured: true, Admit: admit})
		ping := post(srv, "ping", `{}`, secret)
		draft := post(srv, "pull_request", pull(true), secret)
		var ignored struct {
			Ignored string `json:"ignored"`
		}
		must(t, json.Unmarshal(ping.Body.Bytes(), &ignored))
		if ping.Code != http.StatusOK || ignored.Ignored != "ping" || draft.Code != http.StatusOK || admit.calls != 0 {
			t.Fatalf("ping %d %s draft %d calls %d", ping.Code, ignored.Ignored, draft.Code, admit.calls)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		admit := &counting{MemoryStore: reviewtest.NewMemoryStore(3)}
		srv := server.New(discard(), handlers.Webhook{Secret: secret, Configured: true, Admit: admit})
		res := post(srv, "pull_request", `{"action":"opened"}`, secret)
		if res.Code != http.StatusBadRequest || admit.calls != 0 || problemCode(t, res) != "bad_request" {
			t.Fatalf("%d %s calls %d", res.Code, problemCode(t, res), admit.calls)
		}
	})
}

type counting struct {
	*reviewtest.MemoryStore
	calls int
}

func (c *counting) Create(ctx context.Context, req review.Request) (review.Review, bool, error) {
	c.calls++
	return c.MemoryStore.Create(ctx, req)
}

type queued struct {
	ReviewID string `json:"review_id"`
	Created  bool   `json:"created"`
}

func post(srv *server.Server, event, body, signSecret string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/webhook", strings.NewReader(body))
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-Hub-Signature-256", sign([]byte(body), signSecret))
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func sign(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func pull(draft bool) string {
	draftText := "false"
	if draft {
		draftText = "true"
	}
	return `{
		"action":"opened",
		"installation":{"id":9},
		"repository":{"name":"eyeful","clone_url":"https://github.com/acme/eyeful.git","owner":{"login":"acme"}},
		"pull_request":{"number":3,"draft":` + draftText + `,"head":{"ref":"feature","sha":"` + headSHA + `"},"base":{"ref":"main","sha":"` + baseSHA + `"}}
	}`
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func problemCode(t *testing.T, res *httptest.ResponseRecorder) string {
	t.Helper()
	var problem struct {
		Code string `json:"code"`
	}
	must(t, json.Unmarshal(res.Body.Bytes(), &problem))
	return problem.Code
}
