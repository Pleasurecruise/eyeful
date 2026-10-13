package classify

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	jev "github.com/mattn/go-jev"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func New(log *slog.Logger, key string, opts ...jev.ClientOption) *Jev {
	return &Jev{log: log.With(slog.String("component", "classify")), key: key, client: jev.NewClient(append([]jev.ClientOption{jev.WithAPIKey(key)}, opts...)...)}
}

func (j *Jev) Same(ctx context.Context, pairs [][2]workflow.Finding) ([]float64, workflow.Usage, error) {
	if j.key == "" {
		return nil, workflow.Usage{}, ErrNoKey
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
	start := time.Now()
	resp, err := j.client.Evaluate(ctx, map[string]map[string]finding{"findings": shown}, questions)
	j.log.Info("jev run", slog.Int("questions", len(questions)), slog.Int64("milliseconds", time.Since(start).Milliseconds()), slog.Any("error", err))
	if err != nil {
		return nil, workflow.Usage{}, fmt.Errorf("jev: %w", err)
	}
	usage := workflow.Usage{Model: resp.Model, Tokens: int64(resp.Usage.InputTokens + resp.Usage.OutputTokens)}
	same := make([]float64, len(pairs))
	for i := range pairs {
		answer := resp.Answers[fmt.Sprintf("q%d", i)]
		if answer == nil {
			return nil, usage, fmt.Errorf("%w: pair %d of %d", ErrUnanswered, i+1, len(pairs))
		}
		same[i] = answer.Noul
	}
	return same, usage, nil
}
