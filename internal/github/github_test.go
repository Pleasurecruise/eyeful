package github

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/workflow/report"
)

func TestPublish(t *testing.T) {
	marker := "<!-- eyeful review 2 -->"
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
		{"everything posted", `[{"body":"` + marker + `"}]`, `[{"body":"# report\n\n<!-- eyeful review 2 part 1 -->"}]`, 4, false, ""},
		{"another review's marker", `[{"body":"<!-- eyeful review 1 -->"}]`, `[]`, 4, false, "POST /repos/acme/eyeful/pulls/3/reviews\nPOST /repos/acme/eyeful/issues/3/comments"},
		{"a line the diff does not show", `[]`, `[]`, 40, false, "POST /repos/acme/eyeful/issues/3/comments"},
		{"GitHub refuses the review", `[]`, `[]`, 4, true, "POST /repos/acme/eyeful/pulls/3/reviews"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var posts []string
			var body string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
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
			app := New(srv.URL, "ghs_abc", 2)
			app.HTTP = srv.Client()
			src := review.Source{Repo: "acme/eyeful", Number: 3, BaseSHA: strings.Repeat("b", 40), HeadSHA: strings.Repeat("a", 40)}
			err := app.Publish(t.Context(), src, []string{"# report"}, []report.Note{{Path: "a.go", Line: tt.line, Body: "issue: fix"}})
			if tt.reject != (err != nil && strings.Contains(err.Error(), "422")) {
				t.Fatalf("publish: %v", err)
			}
			if strings.Join(posts, "\n") != tt.want {
				t.Fatalf("posted %v", posts)
			}
			if tt.want != "" && !tt.reject && strings.Count(body, "eyeful review 2") != strings.Count(tt.want, "POST") {
				t.Fatalf("every post carries the marker: %s", body)
			}
			if outside := strings.Contains(body, "**`a.go:40`** issue: fix"); outside != (tt.line == 40) {
				t.Fatalf("a note off the diff is not in the comment: %s", body)
			}
		})
	}
}

func TestPages(t *testing.T) {
	got, err := pages([]string{"aaaa", "bb", "cccccc", "d"}, 8)
	if err != nil || !slices.Equal(got, []string{"aaaa\n\nbb", "cccccc", "d"}) {
		t.Fatalf("pages %q, %v", got, err)
	}
	if _, err := pages([]string{"a", "too long for a page"}, 8); !errors.Is(err, ErrTooLong) {
		t.Fatalf("a block over the limit: %v", err)
	}
}

func TestPublishPages(t *testing.T) {
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`[{"body":"first\n\n<!-- eyeful review 2 part 1 -->"}]`))
		default:
			var c struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
				t.Error(err)
			}
			bodies = append(bodies, c.Body)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	app := New(srv.URL, "ghs_abc", 2)
	app.HTTP = srv.Client()
	src := review.Source{Repo: "acme/eyeful", Number: 3, BaseSHA: strings.Repeat("b", 40), HeadSHA: strings.Repeat("a", 40)}
	half := strings.Repeat("x", commentLimit/2)
	if err := app.Publish(t.Context(), src, []string{half, half}, nil); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 1 || !strings.HasSuffix(bodies[0], "<!-- eyeful review 2 part 2 -->") || len(bodies[0]) > commentLimit {
		t.Fatalf("posted %d comments; a restart must post only the page GitHub does not have", len(bodies))
	}
}

func TestTokenRedacts(t *testing.T) {
	var sent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sent = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		http.Error(w, "rejected "+sent, http.StatusInternalServerError)
	}))
	defer srv.Close()
	app := New(srv.URL, "ghs_abc", 0)
	app.HTTP = srv.Client()
	_, err := app.Description(t.Context(), review.Source{Repo: "acme/eyeful", Number: 3})
	if sent != "ghs_abc" || err == nil || strings.Contains(err.Error(), sent) {
		t.Fatalf("error contains the token: %v", err)
	}
}

func TestPull(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/eyeful/pulls/3" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"state":"closed","base":{"ref":"main","sha":"b","repo":{"clone_url":"https://github.com/acme/eyeful.git"}},"head":{"ref":"fix","sha":"a","repo":{"clone_url":"https://github.com/fork/eyeful.git"}}}`))
	}))
	defer srv.Close()
	app := New(srv.URL, "ghs_abc", 0)
	app.HTTP = srv.Client()
	src, open, err := app.Pull(t.Context(), "acme/eyeful", 3)
	want := review.Source{Repo: "acme/eyeful", Base: "main", Head: "fix", Number: 3, BaseSHA: "b", HeadSHA: "a", CloneURL: "https://github.com/acme/eyeful.git"}
	if err != nil || open || src != want {
		t.Fatalf("%+v open %v: %v", src, open, err)
	}
}

func TestWait(t *testing.T) {
	var reads atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/acme/eyeful/actions/runs/2":
			_, _ = w.Write([]byte(`{"id":2,"workflow_id":9}`))
		case strings.HasSuffix(r.URL.Path, "/jobs"):
			_, _ = w.Write([]byte(`{"jobs":[{"name":"test","conclusion":"failure","steps":[{"name":"run","conclusion":"failure"}]}]}`))
		case reads.Add(1) == 1:
			_, _ = w.Write([]byte(`{"workflow_runs":[{"id":1,"workflow_id":5,"name":"ci","status":"in_progress","html_url":"https://example.test/run"},{"id":2,"workflow_id":9,"name":"eyeful","status":"in_progress"}]}`))
		default:
			_, _ = w.Write([]byte(`{"workflow_runs":[{"id":1,"workflow_id":5,"name":"ci","status":"completed","conclusion":"failure","html_url":"https://example.test/run"},{"id":2,"workflow_id":9,"name":"eyeful","status":"in_progress"}]}`))
		}
	}))
	defer srv.Close()
	app := New(srv.URL, "ghs_abc", 2)
	app.HTTP = srv.Client()
	app.poll = time.Millisecond
	src := review.Source{Repo: "acme/eyeful", HeadSHA: strings.Repeat("a", 40)}
	got, err := app.Wait(t.Context(), src)
	if err != nil || got.Passed || !strings.Contains(got.Log, "job test: step run failure") || reads.Load() < 2 {
		t.Fatalf("%+v %v reads %d", got, err, reads.Load())
	}
}

func TestWaitSkipsItsOwnWorkflow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/acme/eyeful/actions/runs/3" {
			_, _ = w.Write([]byte(`{"id":3,"workflow_id":9}`))
			return
		}
		_, _ = w.Write([]byte(`{"workflow_runs":[{"id":1,"workflow_id":5,"name":"ci","status":"completed","conclusion":"success"},{"id":2,"workflow_id":9,"name":"eyeful","status":"completed","conclusion":"failure"},{"id":3,"workflow_id":9,"name":"eyeful","status":"in_progress"}]}`))
	}))
	defer srv.Close()
	app := New(srv.URL, "ghs_abc", 3)
	app.HTTP = srv.Client()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	got, err := app.Wait(ctx, review.Source{Repo: "acme/eyeful", HeadSHA: strings.Repeat("a", 40)})
	if err != nil || !got.Passed {
		t.Fatalf("an earlier eyeful run counted as the project's CI: %+v %v", got, err)
	}
}
