package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/sourcegraph/go-diff/diff"

	"github.com/Pleasurecruise/eyeful/internal/review"
)

type Client struct {
	API   string
	Token string
	Run   int64
	HTTP  *http.Client
	poll  time.Duration
}

func New(api, token string, run int64) *Client {
	return &Client{
		API: strings.TrimRight(api, "/"), Token: token, Run: run,
		HTTP: &http.Client{Timeout: 30 * time.Second},
		poll: 5 * time.Second,
	}
}

func (a *Client) Pull(ctx context.Context, repo string, number int64) (review.Source, bool, error) {
	var out struct {
		State string `json:"state"`
		Base  struct {
			Ref  string `json:"ref"`
			SHA  string `json:"sha"`
			Repo struct {
				CloneURL string `json:"clone_url"`
			} `json:"repo"`
		} `json:"base"`
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	}
	data, err := a.call(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/pulls/%d", repo, number), nil)
	if err != nil {
		return review.Source{}, false, err
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return review.Source{}, false, fmt.Errorf("pull request: %w", err)
	}
	return review.Source{
		Repo: repo, Base: out.Base.Ref, Head: out.Head.Ref, Number: number,
		BaseSHA: out.Base.SHA, HeadSHA: out.Head.SHA, CloneURL: out.Base.Repo.CloneURL,
	}, out.State == "open", nil
}

func (a *Client) Description(ctx context.Context, src review.Source) (string, error) {
	owner, name, _ := strings.Cut(src.Repo, "/")
	var out struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d", owner, name, src.Number)
	data, err := a.call(ctx, http.MethodGet, path, nil)
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

func (a *Client) Wait(ctx context.Context, src review.Source) (Actions, error) {
	var own struct {
		WorkflowID int64 `json:"workflow_id"`
	}
	data, err := a.call(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/actions/runs/%d", src.Repo, a.Run), nil)
	if err != nil {
		return Actions{}, err
	}
	if err := json.Unmarshal(data, &own); err != nil {
		return Actions{}, fmt.Errorf("the reviewing run: %w", err)
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return Actions{}, fmt.Errorf("actions: %w", ctx.Err())
		case <-timer.C:
		}
		actions, pending, err := a.actions(ctx, src, own.WorkflowID)
		if err != nil || !pending {
			return actions, err
		}
		timer.Reset(a.poll)
	}
}

func (a *Client) Publish(ctx context.Context, src review.Source, blocks []string, notes []Note) error {
	owner, name, _ := strings.Cut(src.Repo, "/")
	marker := fmt.Sprintf("<!-- eyeful review %d -->", a.Run)
	if len(notes) > 0 {
		hunks, err := a.hunks(ctx, owner, name, src)
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
		if err := a.once(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews", owner, name, src.Number), marker, func() error {
			return a.review(ctx, owner, name, src, notes, marker)
		}); err != nil {
			return err
		}
	}
	parts, err := pages(blocks, commentLimit-markerRoom)
	if err != nil {
		return err
	}
	for i, page := range parts {
		mark := fmt.Sprintf("<!-- eyeful review %d part %d -->", a.Run, i+1)
		if err := a.once(ctx, fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, name, src.Number), mark, func() error {
			return a.comment(ctx, owner, name, src.Number, page+"\n\n"+mark)
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

func (a *Client) hunks(ctx context.Context, owner, name string, src review.Source) (map[string][]*diff.Hunk, error) {
	var compare struct {
		Files []struct {
			Filename string `json:"filename"`
			Patch    string `json:"patch"`
		} `json:"files"`
	}
	data, err := a.call(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/%s/compare/%s...%s?per_page=1", owner, name, src.BaseSHA, src.HeadSHA), nil)
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

func (a *Client) once(ctx context.Context, path, marker string, post func() error) error {
	for page := 1; ; page++ {
		var bodies []struct {
			Body string `json:"body"`
		}
		data, err := a.call(ctx, http.MethodGet, fmt.Sprintf("%s?per_page=%d&page=%d", path, listPage, page), nil)
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

func (a *Client) actions(ctx context.Context, src review.Source, workflow int64) (Actions, bool, error) {
	owner, name, _ := strings.Cut(src.Repo, "/")
	var page struct {
		WorkflowRuns []struct {
			ID         int64  `json:"id"`
			WorkflowID int64  `json:"workflow_id"`
			Name       string `json:"name"`
			Status     string `json:"status"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
		} `json:"workflow_runs"`
	}
	path := fmt.Sprintf("/repos/%s/%s/actions/runs?head_sha=%s&per_page=30", owner, name, src.HeadSHA)
	data, err := a.call(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Actions{}, false, err
	}
	if err := json.Unmarshal(data, &page); err != nil {
		return Actions{}, false, fmt.Errorf("actions: %w", err)
	}
	var failed []string
	pending := false
	for _, run := range page.WorkflowRuns {
		if run.WorkflowID == workflow {
			continue
		}
		switch run.Status {
		case "completed":
			if failedConclusion(run.Conclusion) {
				failed = append(failed, fmt.Sprintf("%s (%s) conclusion %s", run.Name, run.HTMLURL, run.Conclusion))
				jobs, err := a.jobs(ctx, owner, name, run.ID)
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

func (a *Client) jobs(ctx context.Context, owner, name string, runID int64) ([]string, error) {
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
	data, err := a.call(ctx, http.MethodGet, path, nil)
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

func (a *Client) review(ctx context.Context, owner, name string, src review.Source, notes []Note, marker string) error {
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
	_, err = a.call(ctx, http.MethodPost, path, body)
	return err
}

type statusError struct {
	code int
	body string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("github %d: %s", e.code, e.body)
}

func (a *Client) React(ctx context.Context, src review.Source) error {
	owner, name, _ := strings.Cut(src.Repo, "/")
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/reactions", owner, name, src.Number)
	_, err := a.call(ctx, http.MethodPost, path, []byte(`{"content":"eyes"}`))
	return err
}

func (a *Client) comment(ctx context.Context, owner, name string, number int64, markdown string) error {
	body, err := json.Marshal(map[string]string{"body": markdown})
	if err != nil {
		return fmt.Errorf("encode comment: %w", err)
	}
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, name, number)
	_, err = a.call(ctx, http.MethodPost, path, body)
	return err
}

func (a *Client) call(ctx context.Context, method, path string, body []byte) ([]byte, error) {
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
	req.Header.Set("Authorization", "Bearer "+a.Token)
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
		text := redact(string(data), a.Token)
		if len(text) > 500 {
			text = text[:500]
		}
		return nil, &statusError{code: res.StatusCode, body: text}
	}
	return data, nil
}

func redact(text, secret string) string {
	if secret == "" {
		return text
	}
	return strings.ReplaceAll(text, secret, "[redacted]")
}
