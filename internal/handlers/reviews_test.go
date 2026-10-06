package handlers_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
	"github.com/Pleasurecruise/eyeful/internal/handlers"
	"github.com/Pleasurecruise/eyeful/internal/review/reviewtest"
	"github.com/Pleasurecruise/eyeful/internal/server"
)

const token = "a"

func stubAuth(_ context.Context, bearer, session string) (auth.Principal, error) {
	if session == "s" {
		return auth.Principal{User: auth.User{ID: "usr_a"}, SessionID: "ses_a"}, nil
	}
	switch bearer {
	case "a":
		return auth.Principal{User: auth.User{ID: "usr_a"}, APIKeyID: "key_a"}, nil
	case "b":
		return auth.Principal{User: auth.User{ID: "usr_b"}, APIKeyID: "key_b"}, nil
	}
	return auth.Principal{}, auth.ErrUnauthenticated
}

func newServer(store *reviewtest.MemoryStore) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return server.New(log, server.Options{Authenticate: stubAuth, PublicRoutes: handlers.PublicRoutes},
		handlers.HealthRoutes{Version: "test", Ping: func(context.Context) error { return nil }},
		handlers.DocsRoutes{},
		handlers.ReviewRoutes{Store: store},
		handlers.OAuthRoutes{},
	)
}

func do(t *testing.T, h http.Handler, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode[T handlers.ReviewResponse | handlers.ReviewList | handlers.CurrentSession | handlers.RepositoryList | handlers.RepositoryResponse | handlers.CommitResponse | handlers.GitTreeResponse | handlers.CommitDiffResponse | apperror.Problem](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func problem(t *testing.T, rec *httptest.ResponseRecorder, status int, code apperror.Code) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status %d, want %d: %s", rec.Code, status, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("content type %q", ct)
	}
	if p := decode[apperror.Problem](t, rec); p.Code != code || p.RequestID == "" {
		t.Errorf("problem %+v, want code %s with a request id", p, code)
	}
}

func TestReviews(t *testing.T) {
	store := reviewtest.NewMemoryStore(3)
	h := newServer(store)
	body := `{"source":{"repo":"octo/app","base":"main","head":"feature"}}`

	rec := do(t, h, "POST", "/reviews", body, "Idempotency-Key", "k1")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	created := decode[handlers.ReviewResponse](t, rec)
	if created.Status != "queued" || created.Source.Head != "feature" {
		t.Fatalf("created %+v", created)
	}

	replay := do(t, h, "POST", "/reviews", body, "Idempotency-Key", "k1")
	if decode[handlers.ReviewResponse](t, replay).ID != created.ID || replay.Header().Get("Idempotent-Replayed") != "true" {
		t.Fatal("replay with the same key must return the first review")
	}
	problem(t, do(t, h, "POST", "/reviews", `{"source":{"repo":"octo/app","base":"main","head":"other"}}`, "Idempotency-Key", "k1"),
		http.StatusConflict, apperror.CodeIdempotencyConflict)

	if got := decode[handlers.ReviewResponse](t, do(t, h, "GET", "/reviews/"+created.ID, "")); got.ID != created.ID {
		t.Fatalf("get %+v", got)
	}
	if list := decode[handlers.ReviewList](t, do(t, h, "GET", "/reviews?limit=5", "")); len(list.Items) != 1 {
		t.Fatalf("list %+v", list)
	}

	problem(t, do(t, h, "PATCH", "/reviews/"+created.ID, `{"status":"canceled"}`), http.StatusMethodNotAllowed, apperror.CodeMethodNotAllowed)

	if _, _, err := store.Claim(t.Context(), "worker", time.Minute); err != nil {
		t.Fatal(err)
	}
	problem(t, do(t, h, "DELETE", "/reviews/"+created.ID, ""), http.StatusConflict, apperror.CodeReviewRunning)

	queued := decode[handlers.ReviewResponse](t, do(t, h, "POST", "/reviews", `{"source":{"repo":"octo/app","base":"main","head":"other"}}`))
	if rec := do(t, h, "DELETE", "/reviews/"+queued.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete queued: %d %s", rec.Code, rec.Body)
	}
	problem(t, do(t, h, "GET", "/reviews/"+queued.ID, ""), http.StatusNotFound, apperror.CodeReviewNotFound)
}

func TestProblems(t *testing.T) {
	h := newServer(reviewtest.NewMemoryStore(3))
	problem(t, do(t, h, "GET", "/reviews/r_missing", ""), http.StatusNotFound, apperror.CodeReviewNotFound)
	problem(t, do(t, h, "GET", "/nowhere", ""), http.StatusNotFound, apperror.CodeNotFound)
	for _, source := range []string{
		`{"repo":"octo/app","base":"main"}`,
		`{"repo":"octo/app","base":"main","head":"main"}`,
		`{"repo":"/home/me/app","base":"main","head":"feature"}`,
		`{"repo":"app","base":"main","head":"feature"}`,
	} {
		problem(t, do(t, h, "POST", "/reviews", `{"source":`+source+`}`), http.StatusBadRequest, apperror.CodeBadRequest)
	}
	problem(t, do(t, h, "POST", "/reviews", `{"source":`), http.StatusBadRequest, apperror.CodeBadRequest)
	problem(t, do(t, h, "GET", "/reviews?limit=0", ""), http.StatusBadRequest, apperror.CodeBadRequest)
}

func TestAuth(t *testing.T) {
	h := newServer(reviewtest.NewMemoryStore(3))
	for _, path := range []string{"/reviews", "/reviews/x"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		problem(t, rec, http.StatusUnauthorized, apperror.CodeUnauthenticated)
	}
	for _, route := range handlers.PublicRoutes {
		method, pattern, _ := strings.Cut(route, " ")
		segments := strings.Split(pattern, "/")
		for i, s := range segments {
			if strings.HasPrefix(s, ":") {
				segments[i] = "x"
			}
		}
		path := strings.Join(segments, "/")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
		if rec.Code == http.StatusUnauthorized {
			t.Errorf("public %s demanded authentication", path)
		}
	}
}

func TestConsole(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	assets := fstest.MapFS{"index.html": {Data: []byte("<html>console</html>")}}
	h := server.New(log, server.Options{Authenticate: stubAuth, PublicRoutes: handlers.PublicRoutes, Assets: http.FS(assets)},
		handlers.ReviewRoutes{Store: reviewtest.NewMemoryStore(3)},
	)
	for _, path := range []string{"/", "/settings/keys"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "console") {
			t.Errorf("%s: %d %q, want the console", path, rec.Code, rec.Body)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/reviews", nil))
	problem(t, rec, http.StatusUnauthorized, apperror.CodeUnauthenticated)
}

func TestRateLimit(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := server.New(log, server.Options{Authenticate: stubAuth},
		handlers.ReviewRoutes{Store: reviewtest.NewMemoryStore(3), CreateRateLimits: []echo.MiddlewareFunc{server.RateLimit("review", 2, time.Minute)}},
	)
	body := `{"source":{"repo":"octo/app","base":"main","head":"feature"}}`
	for range 2 {
		if rec := do(t, h, "POST", "/reviews", body); rec.Code != http.StatusAccepted {
			t.Fatalf("create: %d", rec.Code)
		}
	}
	rec := do(t, h, "POST", "/reviews", body)
	problem(t, rec, http.StatusTooManyRequests, apperror.CodeRateLimited)
	if rec.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After")
	}
	if rec := do(t, h, "GET", "/reviews", ""); rec.Code != http.StatusOK {
		t.Errorf("reads share no limit with creates: %d", rec.Code)
	}
}

func TestReviewOwnership(t *testing.T) {
	h := newServer(reviewtest.NewMemoryStore(3))
	created := decode[handlers.ReviewResponse](t, do(t, h, "POST", "/reviews", `{"source":{"repo":"octo/app","base":"main","head":"feature"}}`))
	problem(t, do(t, h, "GET", "/reviews/"+created.ID, "", "Authorization", "Bearer b"), http.StatusNotFound, apperror.CodeReviewNotFound)
	if list := decode[handlers.ReviewList](t, do(t, h, "GET", "/reviews", "", "Authorization", "Bearer b")); len(list.Items) != 0 {
		t.Fatalf("other user sees %d reviews", len(list.Items))
	}
}

func TestCurrentSession(t *testing.T) {
	h := server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), server.Options{Authenticate: stubAuth, PublicRoutes: handlers.PublicRoutes},
		handlers.SessionRoutes{},
	)
	got := decode[handlers.CurrentSession](t, do(t, h, "GET", "/sessions/current", ""))
	if got.User.ID != "usr_a" || got.Via != "api_key" {
		t.Fatalf("session %+v", got)
	}
}

func TestOriginGuard(t *testing.T) {
	h := server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), server.Options{Authenticate: stubAuth, TrustedOrigins: []string{"https://eyeful.example"}},
		handlers.ReviewRoutes{Store: reviewtest.NewMemoryStore(3)},
	)
	cases := map[string]struct {
		origin  string
		session bool
		want    int
	}{
		"cookie from the trusted origin": {"https://eyeful.example", true, http.StatusAccepted},
		"cookie from another origin":     {"https://evil.example", true, http.StatusForbidden},
		"cookie without an origin":       {"", true, http.StatusForbidden},
		"api key from another origin":    {"https://evil.example", false, http.StatusAccepted},
	}
	for name, c := range cases {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/reviews", strings.NewReader(`{"source":{"repo":"octo/app","base":"main","head":"feature"}}`))
		req.Header.Set("Content-Type", "application/json")
		if c.origin != "" {
			req.Header.Set("Origin", c.origin)
		}
		if c.session {
			req.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "s"})
		} else {
			req.Header.Set("Authorization", "Bearer a")
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: %d, want %d: %s", name, rec.Code, c.want, rec.Body)
		}
	}
	read := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/reviews", nil)
	read.Header.Set("Origin", "https://evil.example")
	read.AddCookie(&http.Cookie{Name: auth.SessionCookie, Value: "s"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, read)
	if rec.Code != http.StatusOK {
		t.Errorf("reads are not guarded: %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	h := server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), server.Options{Authenticate: stubAuth, CORSOrigins: []string{"https://console.example"}},
		handlers.ReviewRoutes{Store: reviewtest.NewMemoryStore(3)},
	)
	for origin, allowed := range map[string]bool{"https://console.example": true, "https://evil.example": false} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodOptions, "/reviews", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		got := rec.Header().Get("Access-Control-Allow-Origin")
		if allowed && (got != origin || rec.Header().Get("Access-Control-Allow-Credentials") != "true") {
			t.Errorf("%s: allow-origin %q, credentials %q", origin, got, rec.Header().Get("Access-Control-Allow-Credentials"))
		}
		if !allowed && got != "" {
			t.Errorf("%s must not be allowed, got %q", origin, got)
		}
	}
}
