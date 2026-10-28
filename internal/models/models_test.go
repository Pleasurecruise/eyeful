package models

import (
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
	proxy := httptest.NewServer(Proxy(base, "provider-key"))
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
