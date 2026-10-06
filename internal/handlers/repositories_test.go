package handlers_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-github/v92/github"

	"github.com/Pleasurecruise/eyeful/internal/apperror"
	"github.com/Pleasurecruise/eyeful/internal/auth"
	"github.com/Pleasurecruise/eyeful/internal/handlers"
	"github.com/Pleasurecruise/eyeful/internal/repository"
	"github.com/Pleasurecruise/eyeful/internal/server"
)

const sha = "0123456789abcdef0123456789abcdef01234567"

type stubRepos map[string]error

func (s stubRepos) List(_ context.Context, userID string) ([]*github.Installation, []*github.Repository, error) {
	if userID != "usr_a" {
		return nil, nil, auth.ErrNotLinked
	}
	return []*github.Installation{{ID: new(int64(1)), HTMLURL: new("https://github.com/settings/installations/1")}},
		[]*github.Repository{{FullName: new("octo/app")}}, nil
}

func (s stubRepos) Get(_ context.Context, _, owner, name string) (*github.Repository, error) {
	if err := s[owner+"/"+name]; err != nil {
		return nil, err
	}
	return &github.Repository{Name: new(name), FullName: new(owner + "/" + name), DefaultBranch: new("main")}, nil
}

func (s stubRepos) Commit(context.Context, string, string, string, string) (*github.RepositoryCommit, error) {
	return &github.RepositoryCommit{
		SHA:     new(sha),
		Commit:  &github.Commit{Message: new("fix: y"), Tree: &github.Tree{SHA: new(sha)}},
		Parents: []*github.Commit{{SHA: new(sha)}},
	}, nil
}

func (s stubRepos) Tree(context.Context, string, string, string, string) (*github.Tree, error) {
	return &github.Tree{SHA: new(sha), Entries: []*github.TreeEntry{{Path: new("a.go"), Type: new("blob"), Mode: new("100644"), SHA: new(sha)}}}, nil
}

func (s stubRepos) Diff(context.Context, string, string, string, string) (string, error) {
	return "diff --git a/a.go b/a.go\n", nil
}

func TestRepositories(t *testing.T) {
	h := server.New(slog.New(slog.NewTextHandler(io.Discard, nil)), server.Options{Authenticate: stubAuth},
		handlers.RepositoryRoutes{Repos: stubRepos{
			"octo/missing": repository.ErrNotFound,
			"octo/..":      repository.ErrInvalidName,
			"octo/busy":    &repository.RateLimitedError{RetryAfter: 90 * time.Second},
		}},
	)
	if got := decode[handlers.RepositoryList](t, do(t, h, "GET", "/repositories", "")); len(got.Items) != 1 || len(got.Installations) != 1 || got.Installations[0].Account != nil {
		t.Fatalf("list %+v", got)
	}
	problem(t, do(t, h, "GET", "/repositories", "", "Authorization", "Bearer b"), http.StatusForbidden, apperror.CodeProviderNotLinked)
	if got := decode[handlers.RepositoryResponse](t, do(t, h, "GET", "/repositories/octo/app", "")); got.FullName != "octo/app" || got.DefaultBranch != "main" {
		t.Fatalf("repository %+v", got)
	}
	commit := decode[handlers.CommitResponse](t, do(t, h, "GET", "/repositories/octo/app/commits/main", ""))
	if commit.SHA != sha || commit.Commit.Message != "fix: y" || commit.Commit.Tree.SHA != sha || commit.Commit.Author != nil || len(commit.Parents) != 1 {
		t.Fatalf("commit %+v", commit)
	}
	if got := decode[handlers.GitTreeResponse](t, do(t, h, "GET", "/repositories/octo/app/git/trees/"+sha, "")); len(got.Tree) != 1 || got.Tree[0].Path != "a.go" {
		t.Fatalf("tree %+v", got)
	}
	if got := decode[handlers.CommitDiffResponse](t, do(t, h, "GET", "/repositories/octo/app/commits/"+sha+"/diff", "")); got.Diff == "" {
		t.Fatal("empty diff")
	}

	problem(t, do(t, h, "GET", "/repositories/octo/missing", ""), http.StatusNotFound, apperror.CodeRepositoryNotFound)
	problem(t, do(t, h, "GET", "/repositories/octo/..", ""), http.StatusBadRequest, apperror.CodeBadRequest)
	rec := do(t, h, "GET", "/repositories/octo/busy", "")
	problem(t, rec, http.StatusServiceUnavailable, apperror.CodeUnavailable)
	if rec.Header().Get("Retry-After") != "90" {
		t.Errorf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
}
