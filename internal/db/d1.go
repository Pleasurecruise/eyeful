package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func New(api, account, database, token string) (*Client, error) {
	if account == "" || database == "" || token == "" {
		return nil, errors.New("d1 account, database and api token are required")
	}
	api = strings.TrimRight(api, "/")
	endpoint := api + "/accounts/" + url.PathEscape(account) + "/d1/database/" + url.PathEscape(database) + "/query"
	return &Client{endpoint: endpoint, token: token, http: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	rows, err := c.Query(ctx, "SELECT 1 AS ok")
	if err != nil {
		return err
	}
	if len(rows) != 1 {
		return errors.New("d1 ping returned no row")
	}
	return nil
}

func (c *Client) Exec(ctx context.Context, query string, params ...string) error {
	_, err := c.Query(ctx, query, params...)
	return err
}

func (c *Client) Query(ctx context.Context, query string, params ...string) ([]Row, error) {
	var last error
	for attempt := 1; attempt <= 5; attempt++ {
		rows, err := c.once(ctx, query, params)
		if err == nil || !retryable(err) || attempt == 5 {
			return rows, err
		}
		last = err
		if err := wait(ctx, attempt); err != nil {
			return nil, err
		}
	}
	return nil, last
}

func (c *Client) once(ctx context.Context, query string, params []string) ([]Row, error) {
	body, err := json.Marshal(SingleQuery{SQL: query, Params: params})
	if err != nil {
		return nil, fmt.Errorf("encode d1 query: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("d1 query: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "eyeful")
	res, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("d1 query: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read d1 query: %w", err)
	}
	var parsed QueryResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("d1 query: status %d", res.StatusCode)
	}
	if res.StatusCode >= 300 || !parsed.Success {
		return nil, c.fail(parsed, res.StatusCode)
	}
	return decodeResults(parsed.Result)
}

func (c *Client) fail(parsed QueryResponse, status int) error {
	msg := fmt.Sprintf("d1 query: status %d", status)
	if len(parsed.Errors) > 0 && parsed.Errors[0].Message != "" {
		msg = parsed.Errors[0].Message
	}
	return &d1Error{msg: strings.ReplaceAll(msg, c.token, "[redacted]")}
}

func retryable(err error) bool {
	var target *d1Error
	if !errors.As(err, &target) {
		return false
	}
	msg := target.Error()
	return strings.Contains(msg, "Network connection lost") ||
		strings.Contains(msg, "storage caused object to be reset") ||
		strings.Contains(msg, "reset because its code was updated")
}

func wait(ctx context.Context, attempt int) error {
	timer := time.NewTimer(time.Duration(attempt) * 20 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (e *d1Error) Error() string { return e.msg }

func decodeResults(parts []QueryResult) ([]Row, error) {
	if len(parts) == 0 || len(parts[len(parts)-1].Results) == 0 {
		return []Row{}, nil
	}
	raw := parts[len(parts)-1].Results
	rows := make([]Row, 0, len(raw))
	for _, obj := range raw {
		values := make(map[string]*string, len(obj))
		for name, value := range obj {
			text, ok, err := decodeCell(value)
			if err != nil {
				return nil, fmt.Errorf("column %s: %w", name, err)
			}
			if ok {
				values[name] = &text
			}
		}
		rows = append(rows, Row{values: values})
	}
	return rows, nil
}

func decodeCell(raw json.RawMessage) (string, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false, nil
	}
	if raw[0] == '"' {
		var text string
		if err := json.Unmarshal(raw, &text); err != nil {
			return "", false, fmt.Errorf("decode d1 text: %w", err)
		}
		return text, true, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var number json.Number
	if err := dec.Decode(&number); err != nil {
		return "", false, fmt.Errorf("decode d1 number: %w", err)
	}
	return number.String(), true, nil
}
