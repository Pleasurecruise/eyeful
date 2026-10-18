package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Pleasurecruise/eyeful/internal/review"
	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/project"
)

func NewClient(base string) *Client {
	return &Client{base: strings.TrimSuffix(base, "/"), client: &http.Client{Timeout: 30 * time.Minute}}
}

func (c *Client) Open(ctx context.Context, src review.Source) (project.Config, error) {
	var cfg project.Config
	body, err := json.Marshal(src)
	if err != nil {
		return cfg, fmt.Errorf("sandbox open: %w", err)
	}
	out, err := c.do(ctx, http.MethodPost, "/open", body)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		return cfg, fmt.Errorf("sandbox open: %w", err)
	}
	return cfg, nil
}

func (c *Client) Setup(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodPost, "/setup", nil)
	return err
}

func (c *Client) Read(ctx context.Context, path string) (string, error) {
	out, err := c.do(ctx, http.MethodGet, "/read?path="+url.QueryEscape(path), nil)
	return string(out), err
}

func (c *Client) Test(ctx context.Context, edits []workflow.Edit, t workflow.Target) (workflow.TestRun, error) {
	var run workflow.TestRun
	body, err := json.Marshal(test{Edits: edits, Target: t})
	if err != nil {
		return run, fmt.Errorf("sandbox test: %w", err)
	}
	out, err := c.do(ctx, http.MethodPost, "/test", body)
	if err != nil {
		return run, err
	}
	if err := json.Unmarshal(out, &run); err != nil {
		return run, fmt.Errorf("sandbox test: %w", err)
	}
	return run, nil
}

func (c *Client) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("sandbox %s: %w", path, err)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sandbox %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	out, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("sandbox %s: %w", path, err)
	}
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("sandbox %s: %s: %s", path, res.Status, strings.TrimSpace(string(out)))
	}
	return out, nil
}
