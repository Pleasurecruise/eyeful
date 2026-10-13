package repository

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/go-github/v92/github"
)

var (
	namePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	shaPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
)

func NewService(tokens Tokens, apiBase string) *Service {
	return &Service{tokens: tokens, apiBase: apiBase}
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("github rate limit, retry in %s: %v", e.RetryAfter, e.Err)
}

func (e *RateLimitedError) Unwrap() error { return e.Err }

// List returns the user's installations of the app and every repository they give it.
func (s *Service) List(ctx context.Context, userID string) ([]*github.Installation, []*github.Repository, error) {
	gh, err := s.client(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	return list(ctx, gh)
}

func (s *Service) Get(ctx context.Context, userID, owner, name string) (*github.Repository, error) {
	gh, err := s.open(ctx, userID, owner, name)
	if err != nil {
		return nil, err
	}
	repo, _, err := gh.Repositories.Get(ctx, owner, name)
	return repo, upstream("get repository", err)
}

func (s *Service) Commit(ctx context.Context, userID, owner, name, ref string) (*github.RepositoryCommit, error) {
	if err := validNames(ref); err != nil {
		return nil, err
	}
	gh, err := s.open(ctx, userID, owner, name)
	if err != nil {
		return nil, err
	}
	commit, _, err := gh.Repositories.GetCommit(ctx, owner, name, ref, nil)
	return commit, upstream("get commit", err)
}

func (s *Service) Tree(ctx context.Context, userID, owner, name, sha string) (*github.Tree, error) {
	if !shaPattern.MatchString(sha) {
		return nil, fmt.Errorf("%w: %q is not a full tree sha", ErrInvalidName, sha)
	}
	gh, err := s.open(ctx, userID, owner, name)
	if err != nil {
		return nil, err
	}
	tree, _, err := gh.Git.GetTree(ctx, owner, name, sha, true)
	return tree, upstream("get tree", err)
}

func (s *Service) Diff(ctx context.Context, userID, owner, name, sha string) (string, error) {
	if !shaPattern.MatchString(sha) {
		return "", fmt.Errorf("%w: %q is not a full commit sha", ErrInvalidName, sha)
	}
	gh, err := s.open(ctx, userID, owner, name)
	if err != nil {
		return "", err
	}
	diff, _, err := gh.Repositories.GetCommitRaw(ctx, owner, name, sha, github.RawOptions{Type: github.Diff})
	return diff, upstream("get diff", err)
}

func (s *Service) client(ctx context.Context, userID string) (*github.Client, error) {
	token, err := s.tokens.Token(ctx, userID)
	if err != nil {
		return nil, err
	}
	gh, err := github.NewClient(github.WithAuthToken(token.AccessToken), github.WithURLs(&s.apiBase, nil), github.WithTimeout(30*time.Second))
	if err != nil {
		return nil, fmt.Errorf("github client: %w", err)
	}
	return gh, nil
}

// TODO(repository): remember each user's repositories for a short while instead of listing them on every read.
func (s *Service) open(ctx context.Context, userID, owner, name string) (*github.Client, error) {
	if err := validNames(owner, name); err != nil {
		return nil, err
	}
	gh, err := s.client(ctx, userID)
	if err != nil {
		return nil, err
	}
	_, repos, err := list(ctx, gh)
	if err != nil {
		return nil, err
	}
	for _, r := range repos {
		if strings.EqualFold(r.GetFullName(), owner+"/"+name) {
			return gh, nil
		}
	}
	return nil, fmt.Errorf("%s/%s: %w", owner, name, ErrNotFound)
}

func list(ctx context.Context, gh *github.Client) ([]*github.Installation, []*github.Repository, error) {
	var installs []*github.Installation
	var repos []*github.Repository
	for inst, err := range gh.Apps.ListUserInstallationsIter(ctx, &github.ListOptions{PerPage: 100}) {
		if err != nil {
			return nil, nil, upstream("list installations", err)
		}
		installs = append(installs, inst)
		for repo, err := range gh.Apps.ListUserReposIter(ctx, inst.GetID(), &github.ListOptions{PerPage: 100}) {
			if err != nil {
				return nil, nil, upstream("list repositories", err)
			}
			repos = append(repos, repo)
		}
	}
	return installs, repos, nil
}

func validNames(parts ...string) error {
	for _, p := range parts {
		if !namePattern.MatchString(p) || p == "." || p == ".." {
			return fmt.Errorf("%w: %q", ErrInvalidName, p)
		}
	}
	return nil
}

func upstream(op string, err error) error {
	if err == nil {
		return nil
	}
	if limit, ok := errors.AsType[*github.RateLimitError](err); ok {
		return &RateLimitedError{RetryAfter: time.Until(limit.Rate.Reset.Time), Err: err}
	}
	if abuse, ok := errors.AsType[*github.AbuseRateLimitError](err); ok {
		return &RateLimitedError{RetryAfter: abuse.GetRetryAfter(), Err: err}
	}
	if resp, ok := errors.AsType[*github.ErrorResponse](err); ok && resp.Response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	}
	return fmt.Errorf("%s: %w", op, err)
}
