package models

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func Proxy(upstream *url.URL, token string) http.Handler {
	proxy := &httputil.ReverseProxy{Rewrite: func(r *httputil.ProxyRequest) {
		r.SetURL(upstream)
		r.Out.Header.Set("Authorization", "Bearer "+token)
	}}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
