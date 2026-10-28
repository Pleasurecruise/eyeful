package classify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	jev "github.com/mattn/go-jev"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func New(log *slog.Logger, key string, opts ...jev.ClientOption) (*Jev, error) {
	if key == "" {
		return nil, ErrNoKey
	}
	client := jev.NewClient(append([]jev.ClientOption{jev.WithAPIKey(key), jev.WithTimeout(5 * time.Minute)}, opts...)...)
	return &Jev{log: log.With(slog.String("component", "classify")), client: client}, nil
}

func (j *Jev) Same(ctx context.Context, pairs [][2]workflow.Finding) ([]float64, workflow.Usage, error) {
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
	out, err := j.client.Evaluate(ctx, map[string]map[string]finding{"findings": shown}, questions)
	j.log.Info("jev run", slog.Int("questions", len(questions)), slog.Int64("milliseconds", time.Since(start).Milliseconds()), slog.Any("error", err))
	if apiErr := (*jev.APIError)(nil); errors.As(err, &apiErr) {
		return nil, workflow.Usage{}, fmt.Errorf("%w: %w", ErrRejected, err)
	}
	if err != nil {
		return nil, workflow.Usage{}, fmt.Errorf("jev: %w", err)
	}
	usage := workflow.Usage{Model: out.Model, Tokens: int64(out.Usage.InputTokens + out.Usage.OutputTokens)}
	same := make([]float64, len(pairs))
	for i := range pairs {
		answer := out.Answers[fmt.Sprintf("q%d", i)]
		if answer == nil {
			return nil, usage, fmt.Errorf("%w: pair %d of %d", ErrUnanswered, i+1, len(pairs))
		}
		same[i] = answer.Noul
	}
	return same, usage, nil
}
