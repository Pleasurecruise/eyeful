package github

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

func TestVerify(t *testing.T) {
	body := []byte(`{"zen":"keep it logically awesome"}`)
	secret := "topsecret"
	mac := signature(body, secret)
	if !Verify(body, mac, secret) || Verify(body, mac, "other") || Verify(body, "", secret) || Verify(body, "sha256=abcd", secret) {
		t.Fatal("signature")
	}
}

func TestParse(t *testing.T) {
	const (
		head = "0123456789abcdef0123456789abcdef01234567"
		base = "fedcba9876543210fedcba9876543210fedcba98"
	)
	good := []byte(`{
		"action":"opened",
		"installation":{"id":9},
		"repository":{"name":"eyeful","clone_url":"https://github.com/acme/eyeful.git","owner":{"login":"acme"}},
		"pull_request":{"number":3,"draft":false,"head":{"ref":"feature","sha":"` + head + `"},"base":{"ref":"main","sha":"` + base + `"}}
	}`)
	got, err := Parse("pull_request", good)
	if err != nil || got.Ignore != "" || got.Pull.InstallationID != 9 {
		t.Fatalf("%+v %v", got, err)
	}
	req := Admission(got.Pull)
	if req.IdempotencyKey != "acme/eyeful#3@"+head || req.Source.Repo != "acme/eyeful" || req.Source.HeadSHA != head {
		t.Fatalf("%+v", req)
	}
	for _, tt := range []struct {
		event  string
		body   string
		ignore string
		bad    bool
	}{
		{"ping", `{}`, "ping", false},
		{"push", `{}`, "event push", false},
		{"pull_request", `{"action":"closed"}`, "action closed", false},
		{"pull_request", strings.Replace(string(good), `"opened"`, `"synchronize"`, 1), "", false},
		{"pull_request", strings.Replace(string(good), `"draft":false`, `"draft":true`, 1), "draft", false},
		{"pull_request", strings.Replace(strings.Replace(string(good), `"opened"`, `"ready_for_review"`, 1), `"draft":false`, `"draft":true`, 1), "", false},
		{"pull_request", `{`, "", true},
		{"pull_request", strings.Replace(string(good), `"id":9`, `"id":0`, 1), "", true},
		{"pull_request", strings.Replace(string(good), head, "abc", 1), "", true},
		{"pull_request", strings.Replace(string(good), `"feature"`, `"../x"`, 1), "", true},
		{"pull_request", strings.Replace(string(good), "https://github.com/acme/eyeful.git", "file:///tmp/repo", 1), "", true},
		{"pull_request", strings.Replace(string(good), "https://github.com", "https://user:token@github.com", 1), "", true},
	} {
		delivery, err := Parse(tt.event, []byte(tt.body))
		switch {
		case tt.bad && !errors.Is(err, ErrMalformed):
			t.Fatalf("%s %s: %v", tt.event, tt.body, err)
		case !tt.bad && (err != nil || delivery.Ignore != tt.ignore):
			t.Fatalf("%s: ignore %q err %v", tt.event, delivery.Ignore, err)
		}
	}
}

func TestParseKey(t *testing.T) {
	if _, err := ParseKey("  "); !errors.Is(err, ErrNoKey) {
		t.Fatal(err)
	}
	if _, err := ParseKey("not a key"); errors.Is(err, ErrNoKey) || err == nil {
		t.Fatal(err)
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pkcs1 := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	parsed, err := ParseKey(string(pkcs1))
	if err != nil || !parsed.Equal(key) {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8})
	path := t.TempDir() + "/app.pem"
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	parsed, err = ParseKey(path)
	if err != nil || !parsed.Equal(key) {
		t.Fatal(err)
	}
}

func TestTokenAndPublish(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch {
		case strings.HasSuffix(r.URL.Path, "/access_tokens"):
			_, _ = w.Write([]byte(`{"token":"ghs_abc"}`))
		case strings.Contains(r.URL.Path, "/pulls/3"):
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"message":"line out of diff"}`))
		default:
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	app := New(srv.URL, 12, key)
	app.HTTP = srv.Client()
	token, err := app.Token(t.Context(), 9)
	if err != nil || token != "ghs_abc" {
		t.Fatal(err)
	}
	src := review.Source{Repo: "acme/eyeful", Number: 3, HeadSHA: strings.Repeat("a", 40)}
	err = app.Publish(t.Context(), token, src, "# report", []Note{{Path: "a.go", Line: 4, Body: "issue: fix"}})
	if err == nil || !strings.Contains(err.Error(), "422") {
		t.Fatalf("publish: %v", err)
	}
	if strings.Join(calls, "\n") != "POST /app/installations/9/access_tokens\nPOST /repos/acme/eyeful/pulls/3/reviews" {
		t.Fatalf("%v", calls)
	}
}

func TestTokenRedacts(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		http.Error(w, "rejected "+sent, http.StatusInternalServerError)
	}))
	defer srv.Close()
	app := New(srv.URL, 12, key)
	app.HTTP = srv.Client()
	_, err = app.Token(t.Context(), 9)
	if sent == "" || err == nil || strings.Contains(err.Error(), sent) {
		t.Fatal("error contains the token")
	}
}

func TestWait(t *testing.T) {
	var reads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/jobs") {
			_, _ = w.Write([]byte(`{"jobs":[{"name":"test","conclusion":"failure","steps":[{"name":"run","conclusion":"failure"}]}]}`))
			return
		}
		if reads.Add(1) == 1 {
			_, _ = w.Write([]byte(`{"workflow_runs":[{"id":1,"name":"ci","status":"in_progress","html_url":"https://example.test/run"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"workflow_runs":[{"id":1,"name":"ci","status":"completed","conclusion":"failure","html_url":"https://example.test/run"}]}`))
	}))
	defer srv.Close()
	app := New(srv.URL, 1, nil)
	app.HTTP = srv.Client()
	app.poll = time.Millisecond
	src := review.Source{Repo: "acme/eyeful", HeadSHA: strings.Repeat("a", 40)}
	got, err := app.Wait(t.Context(), "ghs_abc", src)
	if err != nil || got.Passed || !strings.Contains(got.Log, "job test: step run failure") || reads.Load() < 2 {
		t.Fatalf("%+v %v reads %d", got, err, reads.Load())
	}
}

func signature(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
