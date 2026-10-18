package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/local/git"
	"github.com/Pleasurecruise/eyeful/local/subject"
	"github.com/Pleasurecruise/eyeful/local/worktree"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

func NewServer(log *slog.Logger, root string, timeout time.Duration) *Server {
	return &Server{log: log, root: root, timeout: timeout}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	var err error
	switch r.Method + " " + r.URL.Path {
	case "GET /health":
		w.WriteHeader(http.StatusNoContent)
		return
	case "POST /open":
		var src review.Source
		if err = json.NewDecoder(r.Body).Decode(&src); err == nil {
			var cfg project.Config
			if cfg, err = s.open(r.Context(), src); err == nil {
				body, err = json.Marshal(cfg)
			}
		}
	case "POST /setup":
		err = s.with(func(ws *worktree.Workspace) error { return ws.Setup(r.Context()) })
	case "GET /read":
		err = s.with(func(ws *worktree.Workspace) error {
			content, err := ws.Read(r.Context(), r.URL.Query().Get("path"))
			body = []byte(content)
			return err
		})
	case "POST /test":
		var t test
		if err = json.NewDecoder(r.Body).Decode(&t); err == nil {
			err = s.with(func(ws *worktree.Workspace) error {
				run, err := ws.Test(r.Context(), t.Edits, t.Target)
				if err != nil {
					return err
				}
				if body, err = json.Marshal(run); err != nil {
					return fmt.Errorf("encode the test run: %w", err)
				}
				return nil
			})
		}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.log.Warn("sandbox request", slog.String("path", r.URL.Path), slog.Any("error", err))
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if _, err := w.Write(body); err != nil {
		s.log.Warn("write response", slog.String("path", r.URL.Path), slog.Any("error", err))
	}
}

func (s *Server) with(fn func(*worktree.Workspace) error) error {
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil {
		return ErrNotOpen
	}
	return fn(ws)
}

func (s *Server) open(ctx context.Context, src review.Source) (project.Config, error) {
	var cfg project.Config
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tree != nil {
		if err := s.tree.Close(context.WithoutCancel(ctx)); err != nil {
			return cfg, fmt.Errorf("close the last checkout: %w", err)
		}
		s.tree, s.ws = nil, nil
	}
	repo := filepath.Join(s.root, "repo")
	if err := errors.Join(os.RemoveAll(s.root), os.MkdirAll(repo, 0o700)); err != nil {
		return cfg, fmt.Errorf("sandbox directory: %w", err)
	}
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"remote", "add", "origin", src.CloneURL},
		{"fetch", "--no-tags", "origin", fmt.Sprintf("+refs/pull/%d/head:refs/remotes/origin/pr", src.Number), "+refs/heads/" + src.Base + ":refs/remotes/origin/base"},
		{"-c", "core.hooksPath=/dev/null", "checkout", "--quiet", "--detach", src.HeadSHA},
		{"worktree", "add", "--quiet", "--detach", filepath.Join(s.root, "base"), src.BaseSHA},
	} {
		if _, err := git.Run(ctx, repo, nil, args...); err != nil {
			return cfg, err
		}
	}
	cfg, err := project.Detect(filepath.Join(s.root, "base"))
	if err != nil {
		return cfg, fmt.Errorf("project commands: %w", err)
	}
	objects := filepath.Join(s.root, "objects")
	snap, err := subject.Read(ctx, repo, subject.Range{Base: src.BaseSHA, Head: src.HeadSHA}, objects)
	if err != nil {
		return cfg, err
	}
	if s.tree, err = worktree.Open(ctx, s.log, snap, objects); err != nil {
		return cfg, err
	}
	s.ws = worktree.NewWorkspace(s.tree, cfg, s.timeout)
	s.log.Info("opened", slog.String("repo", src.Repo), slog.String("head", src.HeadSHA), slog.String("commands", strings.Join([]string{cfg.Setup, cfg.Test}, " | ")))
	return cfg, nil
}
