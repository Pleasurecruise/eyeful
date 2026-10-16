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
	"strconv"
	"strings"
	"time"

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

func (a *App) Publish(ctx context.Context, token string, src review.Source, markdown string, notes []Note) error {
	owner, name, _ := strings.Cut(src.Repo, "/")
	if len(notes) > 0 {
		err := a.review(ctx, token, owner, name, src, notes)
		if err != nil {
			return err
		}
	}
	return a.comment(ctx, token, owner, name, src.Number, markdown)
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

func (a *App) review(ctx context.Context, token, owner, name string, src review.Source, notes []Note) error {
	comments := make([]reviewComment, 0, len(notes))
	for _, n := range notes {
		comments = append(comments, reviewComment{Path: n.Path, Line: n.Line, Side: "RIGHT", Body: n.Body})
	}
	body, err := json.Marshal(reviewRequest{
		CommitID: src.HeadSHA, Event: "COMMENT", Body: "eyeful posted the ranked report as a comment.", Comments: comments,
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
	data, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
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
