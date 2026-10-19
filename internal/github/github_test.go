package github

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
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

func TestPublish(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	marker := "<!-- eyeful review do/2 -->"
	for _, tt := range []struct {
		name     string
		reviews  string
		comments string
		line     int
		reject   bool
		want     string
	}{
		{"first attempt", `[]`, `[]`, 4, false, "POST /repos/acme/eyeful/pulls/3/reviews\nPOST /repos/acme/eyeful/issues/3/comments"},
		{"review posted before a restart", `[{"body":"x"},{"body":"eyeful posted the ranked report as a comment.\n\n` + marker + `"}]`, `[]`, 4, false, "POST /repos/acme/eyeful/issues/3/comments"},
		{"everything posted", `[{"body":"` + marker + `"}]`, `[{"body":"# report\n\n` + marker + `"}]`, 4, false, ""},
		{"another review's marker", `[{"body":"<!-- eyeful review do/1 -->"}]`, `[]`, 4, false, "POST /repos/acme/eyeful/pulls/3/reviews\nPOST /repos/acme/eyeful/issues/3/comments"},
		{"a line the diff does not show", `[]`, `[]`, 40, false, "POST /repos/acme/eyeful/issues/3/comments"},
		{"GitHub refuses the review", `[]`, `[]`, 4, true, "POST /repos/acme/eyeful/pulls/3/reviews"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var posts []string
			var body string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/access_tokens"):
					_, _ = w.Write([]byte(`{"token":"ghs_abc"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/acme/eyeful/compare/"+strings.Repeat("b", 40)+"..."+strings.Repeat("a", 40):
					_, _ = w.Write([]byte(`{"commits":[],"files":[{"filename":"a.go","patch":"@@ -1,3 +1,5 @@\n a\n+b\n+c\n d\n e"},{"filename":"logo.png"}]}`))
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/reviews"):
					_, _ = w.Write([]byte(tt.reviews))
				case r.Method == http.MethodGet:
					_, _ = w.Write([]byte(tt.comments))
				case tt.reject:
					posts = append(posts, r.Method+" "+r.URL.Path)
					w.WriteHeader(http.StatusUnprocessableEntity)
					_, _ = w.Write([]byte(`{"message":"line out of diff"}`))
				default:
					posts = append(posts, r.Method+" "+r.URL.Path)
					data, _ := io.ReadAll(r.Body)
					body += string(data)
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(`{}`))
				}
			}))
			defer srv.Close()
			app := New(srv.URL, 12, key)
			app.HTTP = srv.Client()
			src := review.Source{Repo: "acme/eyeful", Number: 3, BaseSHA: strings.Repeat("b", 40), HeadSHA: strings.Repeat("a", 40), InstallationID: 9}
			err := app.Publish(t.Context(), src, "do/2", "# report", []Note{{Path: "a.go", Line: tt.line, Body: "issue: fix"}})
			if tt.reject != (err != nil && strings.Contains(err.Error(), "422")) {
				t.Fatalf("publish: %v", err)
			}
			if strings.Join(posts, "\n") != tt.want {
				t.Fatalf("posted %v", posts)
			}
			if tt.want != "" && !tt.reject && strings.Count(body, "eyeful review do/2 --") != strings.Count(tt.want, "POST") {
				t.Fatalf("every post carries the marker: %s", body)
			}
			if outside := strings.Contains(body, "**`a.go:40`** issue: fix"); outside != (tt.line == 40) {
				t.Fatalf("a note off the diff is not in the comment: %s", body)
			}
		})
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
