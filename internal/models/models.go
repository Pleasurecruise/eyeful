package models

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

func Proxy(log *slog.Logger, upstream *url.URL, token string, timeout time.Duration) http.Handler {
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(upstream)
			r.Out.Header.Set("Authorization", "Bearer "+token)
		},
		ModifyResponse: func(res *http.Response) error {
			if res.StatusCode >= http.StatusBadRequest {
				log.Warn("model request failed", slog.Int("status", res.StatusCode), slog.String("retry_after", res.Header.Get("Retry-After")))
			}
			return nil
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		proxy.ServeHTTP(w, r.WithContext(ctx))
	})
}
