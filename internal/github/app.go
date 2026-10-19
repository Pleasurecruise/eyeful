package github

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sourcegraph/go-diff/diff"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

type App struct {
	API   string
	AppID int64
	Key   *rsa.PrivateKey
	HTTP  *http.Client
	poll  time.Duration
}

func New(api string, appID int64, key *rsa.PrivateKey) *App {
	return &App{
		API: strings.TrimRight(api, "/"), AppID: appID, Key: key,
		HTTP: &http.Client{Timeout: 30 * time.Second},
		poll: 5 * time.Second,
	}
}

func (a *App) Token(ctx context.Context, installation int64) (string, error) {
	if a.Key == nil || a.AppID < 1 {
		return "", ErrNoKey
	}
	if installation < 1 {
		return "", fmt.Errorf("installation %d", installation)
	}
	jwt, err := signApp(a.AppID, a.Key, time.Now())
	if err != nil {
		return "", err
	}
	var out struct {
		Token string `json:"token"`
	}
	data, err := a.call(ctx, http.MethodPost, fmt.Sprintf("/app/installations/%d/access_tokens", installation), jwt, []byte("{}"))
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("installation token: %w", err)
	}
	if out.Token == "" {
		return "", fmt.Errorf("installation %d: empty token", installation)
	}
	return out.Token, nil
}

func (a *App) Description(ctx context.Context, token string, src review.Source) (string, error) {
	owner, name, _ := strings.Cut(src.Repo, "/")
	var out struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, name, src.Number)
	data, err := a.call(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return "", err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("pull request: %w", err)
	}
	if out.Body == "" {
		return out.Title, nil
	}
	return out.Title + "\n\n" + out.Body, nil
}

func (a *App) Wait(ctx context.Context, token string, src review.Source) (Actions, error) {
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return Actions{}, fmt.Errorf("actions: %w", ctx.Err())
		case <-timer.C:
		}
		actions, pending, err := a.actions(ctx, token, src)
		if err != nil || !pending {
			return actions, err
		}
		timer.Reset(a.poll)
	}
}

func (a *App) Publish(ctx context.Context, src review.Source, id string, blocks []string, notes []Note) error {
	token, err := a.Token(ctx, src.InstallationID)
	if err != nil {
		return err
	}
	owner, name, _ := strings.Cut(src.Repo, "/")
	marker := fmt.Sprintf("<!-- eyeful review %s -->", id)
	if len(notes) > 0 {
		hunks, err := a.hunks(ctx, token, owner, name, src)
		if err != nil {
			return err
		}
		var inline []Note
		heading := "#### Issues on lines the diff does not show\n\n"
		for _, n := range notes {
			if slices.ContainsFunc(hunks[n.Path], func(h *diff.Hunk) bool {
				return n.Line >= int(h.NewStartLine) && n.Line < int(h.NewStartLine+h.NewLines)
			}) {
				inline = append(inline, n)
				continue
			}
			blocks = append(blocks, fmt.Sprintf("%s**`%s:%d`** %s", heading, n.Path, n.Line, n.Body))
			heading = ""
		}
		notes = inline
	}
	if len(notes) > 0 {
		if err := a.once(ctx, token, fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, name, src.Number), marker, func() error {
			return a.review(ctx, token, owner, name, src, notes, marker)
		}); err != nil {
			return err
		}
	}
	parts, err := pages(blocks, commentLimit-markerRoom)
	if err != nil {
		return err
	}
	for i, page := range parts {
		mark := fmt.Sprintf("<!-- eyeful review %s part %d -->", id, i+1)
		if err := a.once(ctx, token, fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, name, src.Number), mark, func() error {
			return a.comment(ctx, token, owner, name, src.Number, page+"\n\n"+mark)
		}); err != nil {
			return err
		}
	}
	return nil
}

func pages(blocks []string, limit int) ([]string, error) {
	var out []string
	var page strings.Builder
	for _, b := range blocks {
		if len(b) > limit {
			return nil, fmt.Errorf("%w: a block of %d bytes, GitHub takes %d", ErrTooLong, len(b), limit)
		}
		if page.Len() > 0 && page.Len()+len("\n\n")+len(b) > limit {
			out = append(out, page.String())
			page.Reset()
		}
		if page.Len() > 0 {
			page.WriteString("\n\n")
		}
		page.WriteString(b)
	}
	return append(out, page.String()), nil
}

func (a *App) hunks(ctx context.Context, token, owner, name string, src review.Source) (map[string][]*diff.Hunk, error) {
	var compare struct {
		Files []struct {
			Filename string `json:"filename"`
			Patch    string `json:"patch"`
		} `json:"files"`
	}
	data, err := a.call(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/compare/%s...%s?per_page=1", owner, name, src.BaseSHA, src.HeadSHA), token, nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &compare); err != nil {
		return nil, fmt.Errorf("compare %.12s...%.12s: %w", src.BaseSHA, src.HeadSHA, err)
	}
	out := map[string][]*diff.Hunk{}
	for _, f := range compare.Files {
		if out[f.Filename], err = diff.ParseHunks([]byte(f.Patch)); err != nil {
			return nil, fmt.Errorf("the patch of %s: %w", f.Filename, err)
		}
	}
	return out, nil
}

func (a *App) once(ctx context.Context, token, path, marker string, post func() error) error {
	for page := 1; ; page++ {
		var bodies []struct {
			Body string `json:"body"`
		}
		data, err := a.call(ctx, http.MethodGet, fmt.Sprintf("%s?per_page=%d&page=%d", path, listPage, page), token, nil)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &bodies); err != nil {
			return fmt.Errorf("list %s: %w", path, err)
		}
		for _, b := range bodies {
			if strings.Contains(b.Body, marker) {
				return nil
			}
		}
		if len(bodies) < listPage {
			return post()
		}
	}
}

func (a *App) actions(ctx context.Context, token string, src review.Source) (Actions, bool, error) {
	owner, name, _ := strings.Cut(src.Repo, "/")
	var page struct {
		WorkflowRuns []struct {
			ID         int64  `json:"id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
		} `json:"workflow_runs"`
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/runs?head_sha=%s&per_page=30", owner, name, src.HeadSHA)
	data, err := a.call(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return Actions{}, false, err
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return Actions{}, false, fmt.Errorf("actions: %w", err)
	}
	var failed []string
	pending := false
	for _, run := range page.WorkflowRuns {
		switch run.Status {
		case "completed":
			if failedConclusion(run.Conclusion) {
				failed = append(failed, fmt.Sprintf("%s (%s) conclusion %s", run.Name, run.HTMLURL, run.Conclusion))
				jobs, err := a.jobs(ctx, token, owner, name, run.ID)
				if err != nil {
					return Actions{}, false, err
				}
				failed = append(failed, jobs...)
			}
		default:
			pending = true
		}
	}
	if pending && len(failed) == 0 {
		return Actions{}, true, nil
	}
	return Actions{Passed: len(failed) == 0, Log: strings.Join(failed, "\n")}, false, nil
}

func (a *App) jobs(ctx context.Context, token, owner, name string, runID int64) ([]string, error) {
	var page struct {
		Jobs []struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
			Steps      []struct {
				Name       string `json:"name"`
				Conclusion string `json:"conclusion"`
			} `json:"steps"`
		} `json:"jobs"`
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/runs/%d/jobs", owner, name, runID)
	data, err := a.call(ctx, http.MethodGet, path, token, nil)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("actions jobs: %w", err)
	}
	var lines []string
	for _, job := range page.Jobs {
		if !failedConclusion(job.Conclusion) {
			continue
		}
		for _, step := range job.Steps {
			if failedConclusion(step.Conclusion) {
				lines = append(lines, fmt.Sprintf("job %s: step %s %s", job.Name, step.Name, step.Conclusion))
			}
		}
	}
	return lines, nil
}

func failedConclusion(value string) bool {
	switch value {
	case "failure", "timed_out", "startup_failure", "cancelled", "action_required":
		return true
	default:
		return false
	}
}

type reviewRequest struct {
	CommitID string          `json:"commit_id"`
	Event    string          `json:"event"`
	Body     string          `json:"body"`
	Comments []reviewComment `json:"comments"`
}

type reviewComment struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Side string `json:"side"`
	Body string `json:"body"`
}

func (a *App) review(ctx context.Context, token, owner, name string, src review.Source, notes []Note, marker string) error {
	comments := make([]reviewComment, 0, len(notes))
	for _, n := range notes {
		comments = append(comments, reviewComment{Path: n.Path, Line: n.Line, Side: "RIGHT", Body: n.Body})
	}
	body, err := json.Marshal(reviewRequest{
		CommitID: src.HeadSHA, Event: "COMMENT", Body: "eyeful posted the ranked report as a comment.\n\n" + marker, Comments: comments,
	})
	if err != nil {
		return fmt.Errorf("encode review: %w", err)
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, name, src.Number)
	_, err = a.call(ctx, http.MethodPost, path, token, body)
	return err
}

type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("github %d: %s", e.code, e.body)
}

func (a *App) React(ctx context.Context, token string, src review.Source) error {
	owner, name, _ := strings.Cut(src.Repo, "/")
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/reactions", owner, name, src.Number)
	_, err := a.call(ctx, http.MethodPost, path, token, []byte(`{"content":"eyes"}`))
	return err
}

func (a *App) comment(ctx context.Context, token, owner, name string, number int64, markdown string) error {
	body, err := json.Marshal(map[string]string{"body": markdown})
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, name, number)
	_, err = a.call(ctx, http.MethodPost, path, token, body)
	return err
}

func (a *App) call(ctx context.Context, method, path, token string, body []byte) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, a.API+path, reader)
	if err != nil {
		return nil, fmt.Errorf("github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "eyeful")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github %s %s: %w", method, path, err)
	}
	defer func() { _ = res.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(res.Body, bodyLimit))
	if err != nil {
		return nil, fmt.Errorf("github %s %s: %w", method, path, err)
	}
	if res.StatusCode >= 300 {
		text := redact(string(data), token)
		if len(text) > 500 {
			text = text[:500]
		}
		return nil, &statusError{code: res.StatusCode, body: text}
	}
	return data, nil
}

func signApp(appID int64, key *rsa.PrivateKey, now time.Time) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := fmt.Sprintf(`{"iat":%d,"exp":%d,"iss":%q}`, now.Add(-time.Minute).Unix(), now.Add(9*time.Minute).Unix(), strconv.FormatInt(appID, 10))
	payload := header + "." + base64.RawURLEncoding.EncodeToString([]byte(claims))
	sum := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", fmt.Errorf("sign GitHub App token: %w", err)
	}
	return payload + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func redact(text, secret string) string {
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "[redacted]")
}
