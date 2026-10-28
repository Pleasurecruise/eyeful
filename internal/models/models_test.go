package models

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestProxy(t *testing.T) {
	var path, auth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer upstream.Close()
	base, err := url.Parse(upstream.URL + "/api/v3")
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(Proxy(slog.New(slog.DiscardHandler), base, "provider-key"))
	defer proxy.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, proxy.URL+"/chat/completions", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer eyeful")
	res, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK || path != "/api/v3/chat/completions" || auth != "Bearer provider-key" {
		t.Fatalf("status %d path %s auth %q", res.StatusCode, path, auth)
	}
	for _, target := range []string{"GET /chat/completions", "POST /models", "POST /../chat/completions"} {
		method, p, _ := strings.Cut(target, " ")
		req, err := http.NewRequestWithContext(t.Context(), method, proxy.URL+p, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := proxy.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Fatalf("%s reached the provider: %d", target, res.StatusCode)
		}
	}
}

func TestProxyLogsFailures(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "30")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer upstream.Close()
	base, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	var logs strings.Builder
	proxy := httptest.NewServer(Proxy(slog.New(slog.NewTextHandler(&logs, nil)), base, "provider-key"))
	defer proxy.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, proxy.URL+"/chat/completions", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	res, err := proxy.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusTooManyRequests || !strings.Contains(logs.String(), "status=429") || !strings.Contains(logs.String(), "retry_after=30") || strings.Contains(logs.String(), "provider-key") {
		t.Fatalf("status %d, logs %q", res.StatusCode, logs.String())
	}
}
