package repository_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/Pleasurecruise/eyeful/internal/repository"
)

const (
	sha   = "0123456789abcdef0123456789abcdef01234567"
	patch = "diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-x\n+y\n"
)

type tokens map[string]string

func (t tokens) Token(_ context.Context, userID string) (*oauth2.Token, error) {
	tok, ok := t[userID]
	if !ok {
		return nil, errors.New("no token")
	}
	return &oauth2.Token{AccessToken: tok}, nil
}

func newService(t *testing.T, h http.Handler) *repository.Service {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return repository.NewService(tokens{"usr_a": "user-token"}, srv.URL+"/")
}

func fakeGitHub(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	user := http.NewServeMux()
	user.HandleFunc("GET /user/installations", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total_count":1,"installations":[{"id":1,"html_url":"https://github.com/settings/installations/1","account":{"login":"octo"}}]}`))
	})
	user.HandleFunc("GET /user/installations/1/repositories", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total_count":1,"repositories":[{"full_name":"octo/app","default_branch":"main"}]}`))
	})
	user.HandleFunc("GET /repos/octo/app", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"full_name":"octo/app","default_branch":"main"}`))
	})
	user.HandleFunc("GET /repos/octo/app/commits/{ref}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") == "application/vnd.github.v3.diff" {
			_, _ = w.Write([]byte(patch))
			return
		}
		_, _ = w.Write([]byte(`{"sha":"` + sha + `","commit":{"message":"fix: y","tree":{"sha":"` + sha + `"}}}`))
	})
	user.HandleFunc("GET /repos/octo/app/git/trees/"+sha, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("recursive") == "" {
			http.Error(w, "want recursive", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"sha":"` + sha + `","truncated":false,"tree":[{"path":"a.go","type":"blob"}]}`))
	})
	user.HandleFunc("GET /repos/octo/public", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"full_name":"octo/public"}`))
	})
	mux.Handle("/user/", authorized(t, user))
	mux.Handle("/repos/", authorized(t, user))
	return mux
}

func authorized(t *testing.T, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer user-token" {
			t.Errorf("%s %s without the user's token", r.Method, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func TestReads(t *testing.T) {
	ctx := context.Background()
	s := newService(t, fakeGitHub(t))
	installs, repos, err := s.List(ctx, "usr_a")
	if err != nil || len(repos) != 1 || repos[0].GetFullName() != "octo/app" {
		t.Fatalf("list %+v, err = %v", repos, err)
	}
	if len(installs) != 1 || installs[0].GetHTMLURL() != "https://github.com/settings/installations/1" {
		t.Errorf("installations %+v", installs)
	}
	repo, err := s.Get(ctx, "usr_a", "octo", "app")
	if err != nil || repo.GetDefaultBranch() != "main" {
		t.Fatalf("repository %+v, err = %v", repo, err)
	}
	commit, err := s.Commit(ctx, "usr_a", "octo", "app", "main")
	if err != nil || commit.GetSHA() != sha || commit.GetCommit().GetTree().GetSHA() != sha {
		t.Fatalf("commit %+v, err = %v", commit, err)
	}
	tree, err := s.Tree(ctx, "usr_a", "octo", "app", sha)
	if err != nil || len(tree.Entries) != 1 {
		t.Fatalf("tree %+v, err = %v", tree, err)
	}
	diff, err := s.Diff(ctx, "usr_a", "octo", "app", sha)
	if err != nil || diff != patch {
		t.Fatalf("diff %q, err = %v", diff, err)
	}
}

func TestAccess(t *testing.T) {
	ctx := context.Background()
	s := newService(t, fakeGitHub(t))
	if _, err := s.Get(ctx, "usr_a", "octo", "public"); !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("a readable repository the user did not give eyeful: err = %v", err)
	}
	if _, _, err := s.List(ctx, "usr_b"); err == nil {
		t.Error("a user without a token listed repositories")
	}
	for _, name := range []string{"..", ".", "a/b", "a?b", ""} {
		if _, err := s.Get(ctx, "usr_a", "octo", name); !errors.Is(err, repository.ErrInvalidName) {
			t.Errorf("name %q: err = %v", name, err)
		}
	}
	for _, ref := range []string{"..", "feature/x", "a%2Fb"} {
		if _, err := s.Commit(ctx, "usr_a", "octo", "app", ref); !errors.Is(err, repository.ErrInvalidName) {
			t.Errorf("ref %q: err = %v", ref, err)
		}
	}
	for _, bad := range []string{"main", sha[:39], sha + "0", "../" + sha[3:]} {
		if _, err := s.Tree(ctx, "usr_a", "octo", "app", bad); !errors.Is(err, repository.ErrInvalidName) {
			t.Errorf("tree sha %q: err = %v", bad, err)
		}
		if _, err := s.Diff(ctx, "usr_a", "octo", "app", bad); !errors.Is(err, repository.ErrInvalidName) {
			t.Errorf("diff sha %q: err = %v", bad, err)
		}
	}
}

func TestRateLimit(t *testing.T) {
	reset := time.Now().Add(time.Hour).Unix()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	})
	_, _, err := newService(t, mux).List(context.Background(), "usr_a")
	var rl *repository.RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter < 59*time.Minute {
		t.Fatalf("rate limited: err = %v", err)
	}
}
