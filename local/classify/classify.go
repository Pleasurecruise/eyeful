package classify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"time"

	jev "github.com/mattn/go-jev"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func New(log *slog.Logger, url string, header http.Header) *Jev {
	return &Jev{log: log.With(slog.String("component", "classify")), url: url, header: header, client: &http.Client{Timeout: 5 * time.Minute}}
}

func (j *Jev) Same(ctx context.Context, pairs [][2]workflow.Finding) ([]float64, workflow.Usage, error) {
	if j.url == "" {
		return nil, workflow.Usage{}, ErrNotConfigured
	}
	shown, keys, questions := map[string]finding{}, map[string]string{}, map[string]jev.Question{}
	for i, p := range pairs {
		for _, f := range p {
			if keys[f.ID] == "" {
				keys[f.ID] = fmt.Sprintf("f%d", len(keys))
				shown[keys[f.ID]] = finding{File: f.Path, Line: f.Line, Subject: f.Subject, Discussion: f.Discussion}
			}
		}
		questions[fmt.Sprintf("q%d", i)] = jev.Question{Type: "noul", Instructions: fmt.Sprintf(sameQuestion, keys[p[0].ID], keys[p[1].ID])}
	}
	body, err := json.Marshal(input{State: map[string]map[string]finding{"findings": shown}, Questions: questions})
	if err != nil {
		return nil, workflow.Usage{}, fmt.Errorf("jev: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.url, bytes.NewReader(body))
	if err != nil {
		return nil, workflow.Usage{}, fmt.Errorf("jev: %w", err)
	}
	maps.Copy(req.Header, j.header)
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	res, err := j.client.Do(req)
	j.log.Info("jev run", slog.Int("questions", len(questions)), slog.Int64("milliseconds", time.Since(start).Milliseconds()), slog.Any("error", err))
	if err != nil {
		return nil, workflow.Usage{}, fmt.Errorf("jev: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	var out reply
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, workflow.Usage{}, fmt.Errorf("jev: %s: %w", res.Status, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, workflow.Usage{}, fmt.Errorf("%w: %s: %+v", ErrRejected, res.Status, out.Errors)
	}
	usage := workflow.Usage{Model: out.Result.Model, Tokens: int64(out.Result.Usage.InputTokens + out.Result.Usage.OutputTokens)}
	same := make([]float64, len(pairs))
	for i := range pairs {
		answer := out.Result.Answers[fmt.Sprintf("q%d", i)]
		if answer == nil {
			return nil, usage, fmt.Errorf("%w: pair %d of %d", ErrUnanswered, i+1, len(pairs))
		}
		same[i] = answer.Noul
	}
	return same, usage, nil
}
