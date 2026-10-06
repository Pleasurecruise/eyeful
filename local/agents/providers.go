package agents

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/Pleasurecruise/eyeful/workflow"
)

var Providers = []Provider{
	{
		Name: "claude", Tool: "claude", Login: "run `claude` and sign in with /login", strip: []string{"ANTHROPIC_API_KEY"},
		args: func(schema []byte, tier workflow.Tier) []string {
			args := []string{"-p", "--output-format", "stream-json", "--verbose", "--include-partial-messages", "--json-schema", string(schema), "--no-session-persistence", "--strict-mcp-config", "--setting-sources", "project", "--disable-slash-commands"}
			if model, ok := map[workflow.Tier]string{workflow.TierCheap: "haiku", workflow.TierMid: "sonnet", workflow.TierStrong: "opus"}[tier]; ok {
				args = append(args, "--model", model)
			}
			return append(args, "--tools", "Read,Grep,Glob")
		},
		parse: func(stdout []byte) (answer, error) {
			var r claudeResult
			lines := bufio.NewScanner(bytes.NewReader(stdout))
			lines.Buffer(nil, len(stdout)+1)
			for lines.Scan() {
				var e claudeEvent
				if err := json.Unmarshal(lines.Bytes(), &e); err != nil {
					return answer{}, fmt.Errorf("read claude's events: %w", err)
				}
				if e.Type != "result" {
					continue
				}
				if err := json.Unmarshal(lines.Bytes(), &r); err != nil {
					return answer{}, fmt.Errorf("read claude's result: %w", err)
				}
			}
			if err := lines.Err(); err != nil {
				return answer{}, fmt.Errorf("read claude's events: %w", err)
			}
			if r.Type != "result" {
				return answer{}, fmt.Errorf("%w: claude ended without a result", ErrFailed)
			}
			u := r.Usage
			a := answer{text: r.Result, usage: workflow.Usage{
				Model:    strings.Join(slices.Sorted(maps.Keys(r.ModelUsage)), ", "),
				Tokens:   u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens,
				MicroUSD: int64(r.TotalCostUSD * 1e6),
			}}
			if len(r.StructuredOutput) > 0 {
				a.text = string(r.StructuredOutput)
			}
			if r.IsError {
				return a, fmt.Errorf("%w: claude ended with %s: %s", ErrFailed, r.Subtype, r.Result)
			}
			return a, nil
		},
	},
	{
		Name: "codex", Tool: "codex", Login: "run `codex login`",
		args: func([]byte, workflow.Tier) []string {
			return []string{
				"exec", "--json", "--sandbox", "read-only", "--skip-git-repo-check", "--ephemeral",
				"--ignore-user-config", "--ignore-rules", "--disable", "hooks", "--disable", "plugins", "--disable", "apps",
				"--enable", "skip_host_skill_discovery", "-c", "project_doc_max_bytes=0",
				"-",
			}
		},
		parse: func(stdout []byte) (answer, error) {
			a := answer{usage: workflow.Usage{Model: "codex"}}
			lines := bufio.NewScanner(bytes.NewReader(stdout))
			lines.Buffer(nil, len(stdout)+1)
			for lines.Scan() {
				var e codexEvent
				if err := json.Unmarshal(lines.Bytes(), &e); err != nil {
					return a, fmt.Errorf("read codex's events: %w", err)
				}
				switch {
				case e.Type == "item.completed" && e.Item.Type == "agent_message":
					a.text = e.Item.Text
				case e.Type == "turn.completed":
					a.usage.Tokens += e.Usage.InputTokens + e.Usage.OutputTokens
				case e.Type == "turn.failed":
					return a, fmt.Errorf("%w: %s", ErrFailed, e.Error.Message)
				case e.Type == "error":
					return a, fmt.Errorf("%w: %s", ErrFailed, e.Message)
				}
			}
			return a, lines.Err()
		},
	},
	{
		Name: "pi", Tool: "pi", Login: "run `pi` and sign in with /login",
		args: func([]byte, workflow.Tier) []string {
			return []string{
				"--mode", "json", "--no-session", "--tools", "read,grep,find,ls",
				"--no-mcp", "--no-extensions", "--no-skills", "--no-prompt-templates", "--no-context-files", "--no-approve",
				"Follow the instructions above.",
			}
		},
		parse: func(stdout []byte) (answer, error) {
			a := answer{usage: workflow.Usage{Model: "pi"}}
			cost := 0.0
			lines := bufio.NewScanner(bytes.NewReader(stdout))
			lines.Buffer(nil, len(stdout)+1)
			for lines.Scan() {
				var e piEvent
				if err := json.Unmarshal(lines.Bytes(), &e); err != nil {
					return a, fmt.Errorf("read pi's events: %w", err)
				}
				m := e.Message
				if e.Type != "message_end" || m.Role != "assistant" {
					continue
				}
				if m.StopReason == "error" {
					return a, fmt.Errorf("%w: %s", ErrFailed, m.ErrorMessage)
				}
				var blocks []piContent
				if err := json.Unmarshal(m.Content, &blocks); err != nil {
					return a, fmt.Errorf("read pi's message: %w", err)
				}
				var text strings.Builder
				for _, b := range blocks {
					if b.Type == "text" {
						text.WriteString(b.Text)
					}
				}
				a.text = text.String()
				a.usage.Model = m.Provider + "/" + m.Model
				a.usage.Tokens += m.Usage.TotalTokens
				cost += m.Usage.Cost.Total
			}
			a.usage.MicroUSD = int64(cost * 1e6)
			return a, lines.Err()
		},
	},
}
