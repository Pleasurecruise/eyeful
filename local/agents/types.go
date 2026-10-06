package agents

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/Pleasurecruise/eyeful/workflow"
	"github.com/Pleasurecruise/eyeful/workflow/prompts"
)

// TODO(agents): pick models for codex and pi tiers once each has a stable catalog to map to.
type Provider struct {
	Name  string
	Tool  string
	Login string
	strip []string
	args  func(schema []byte, tier workflow.Tier) []string
	parse func(stdout []byte) (answer, error)
}

type answer struct {
	text  string
	usage workflow.Usage
}

type Config struct {
	Provider Provider
	Dir      string
	Skills   string
	Set      prompts.Set
}

type activity struct {
	out   *bytes.Buffer
	idle  *time.Timer
	after time.Duration
}

type Agents struct {
	cfg  Config
	log  *slog.Logger
	idle time.Duration
	hard time.Duration
}

type Agent struct {
	Name    string
	Version string
}

type fixReply struct {
	Edits []workflow.Edit `json:"edits"`
}

type commitReply struct {
	Message string `json:"message"`
}

type summaryReply struct {
	Comments []workflow.Draft `json:"comments"`
}

type reply interface {
	workflow.Diagnosis | workflow.Plan | workflow.Report | fixReply | workflow.Judgement | summaryReply | commitReply
}

type claudeEvent struct {
	Type string `json:"type"`
}

type claudeResult struct {
	Type             string                     `json:"type"`
	Subtype          string                     `json:"subtype"`
	IsError          bool                       `json:"is_error"`
	Result           string                     `json:"result"`
	StructuredOutput json.RawMessage            `json:"structured_output"`
	TotalCostUSD     float64                    `json:"total_cost_usd"`
	Usage            claudeUsage                `json:"usage"`
	ModelUsage       map[string]json.RawMessage `json:"modelUsage"`
}

type claudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

type codexEvent struct {
	Type    string     `json:"type"`
	Message string     `json:"message"`
	Usage   codexUsage `json:"usage"`
	Item    codexItem  `json:"item"`
	Error   codexError `json:"error"`
}

type codexUsage struct {
	InputTokens       int64 `json:"input_tokens"`
	CachedInputTokens int64 `json:"cached_input_tokens"`
	OutputTokens      int64 `json:"output_tokens"`
}

type codexItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type codexError struct {
	Message string `json:"message"`
}

type piEvent struct {
	Type    string    `json:"type"`
	Message piMessage `json:"message"`
}

type piMessage struct {
	Role         string          `json:"role"`
	Content      json.RawMessage `json:"content"`
	Provider     string          `json:"provider"`
	Model        string          `json:"model"`
	Usage        piUsage         `json:"usage"`
	StopReason   string          `json:"stopReason"`
	ErrorMessage string          `json:"errorMessage"`
}

type piContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type piUsage struct {
	TotalTokens int64  `json:"totalTokens"`
	Cost        piCost `json:"cost"`
}

type piCost struct {
	Total float64 `json:"total"`
}

const (
	stderrLimit = 4 << 10
	commitSkill = "git-commit"
)

var (
	ErrBadReply     = errors.New("the agent's reply is not the result asked for")
	ErrFailed       = errors.New("the agent failed")
	ErrNotInstalled = errors.New("agent not installed")
	ErrIdle         = errors.New("the agent wrote nothing for too long")
	ErrHardTimeout  = errors.New("the agent ran for too long")
)
