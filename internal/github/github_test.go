package github

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
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
	src := review.Source{Repo: "acme/eyeful", Number: 3, HeadSHA: strings.Repeat("a", 40), InstallationID: 9}
	err = app.Publish(t.Context(), src, "# report", []Note{{Path: "a.go", Line: 4, Body: "issue: fix"}})
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
