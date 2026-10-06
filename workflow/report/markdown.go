package report

import (
	"fmt"
	"strings"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func Markdown(r workflow.Result) (string, error) {
	out, err := Findings(r)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# eyeful review\n\n%d findings: %d important, %d nit. Level `%s`, outcome `%s`.\n", len(out.Findings), out.Counts.Important, out.Counts.Nit, out.Level, out.Outcome)
	if out.Reason != "" {
		fmt.Fprintf(&b, "%s.\n", out.Reason)
	}
	if out.Outcome == workflow.OutcomeCIFailed {
		b.WriteString("\n## CI failed\n\n")
		for _, f := range out.Diagnosis.Failures {
			fmt.Fprintf(&b, "- `%s` %s %s: %s\n  Fix: %s\n", f.Workflow, f.Job, f.Step, f.Cause, f.Fix)
		}
	}
	if out.Counts.Important == 0 && out.Outcome != workflow.OutcomeCIFailed {
		b.WriteString("\nNo blocking issues.\n")
	}
	if len(out.Findings) > 0 {
		b.WriteString("\n| Severity | File:Line | Issue |\n| --- | --- | --- |\n")
		for _, f := range out.Findings {
			fmt.Fprintf(&b, "| %s | `%s:%d` | %s |\n", markers[f.Severity], f.File, f.Line, f.ShortSummary)
		}
	}
	for _, f := range out.Findings {
		finding(&b, f)
	}
	if len(r.Scopes) > 1 {
		fmt.Fprintf(&b, "\n## Scopes\n\nThe change was reviewed in %d scopes, in this order. Each could be its own change in a stack.\n\n| Scope | Files | Lines |\n| --- | --- | --- |\n", len(r.Scopes))
		for _, s := range r.Scopes {
			fmt.Fprintf(&b, "| `%s` | %d | %d |\n", s.Name, len(s.Files), s.Lines)
		}
	}
	if len(out.NotReviewed) > 0 {
		b.WriteString("\n## Not reviewed\n\n")
		for _, e := range out.NotReviewed {
			fmt.Fprintf(&b, "- `%s` (%s)\n", e.Path, e.Class)
		}
	}
	uncertainty(&b, r)
	return b.String(), nil
}

func finding(b *strings.Builder, f Finding) {
	fmt.Fprintf(b, "\n## %s · %s · %s\n\n`%s:%d` %s\n", markers[f.Severity], f.Category, f.Verdict, f.File, f.Line, f.Summary)
	if f.FailureScenario != "" {
		fmt.Fprintf(b, "\nFailure scenario: %s\n", f.FailureScenario)
	}
	b.WriteString("\n<details><summary>Why this was flagged</summary>\n\n")
	if f.Why != "" {
		fmt.Fprintf(b, "%s\n\n", f.Why)
	}
	fmt.Fprintf(b, "%s Reported by %s.\n", f.Verification, strings.Join(f.Experts, ", "))
	if f.Test != nil {
		fmt.Fprintf(b, "\n`%s`:\n\n```\n%s\n```\n", f.Test.Path, strings.TrimRight(f.Test.New, "\n"))
	}
	b.WriteString("\n</details>\n")
	for _, e := range f.Fix {
		fmt.Fprintf(b, "\nVerified fix in `%s`:\n\n```diff\n", e.Path)
		for line := range strings.Lines(strings.TrimRight(e.Old, "\n") + "\n") {
			b.WriteString("-" + line)
		}
		for line := range strings.Lines(strings.TrimRight(e.New, "\n") + "\n") {
			b.WriteString("+" + line)
		}
		b.WriteString("```\n")
	}
}

func uncertainty(b *strings.Builder, r workflow.Result) {
	u := r.Uncertainty
	var lines []string
	for _, a := range u.Absent {
		lines = append(lines, fmt.Sprintf("Expert `%s` on group %d is absent: %s", a.Expert, a.Group, a.Reason))
	}
	for _, s := range u.Unread {
		lines = append(lines, fmt.Sprintf("Expert `%s` on group %d did not load skill `%s`", s.Expert, s.Group, s.Skill))
	}
	for _, f := range u.Dismissed {
		lines = append(lines, fmt.Sprintf("Dismissed, not reproduced: `%s:%d` %s (%s)", f.Path, f.Line, f.Subject, f.Expert))
	}
	for _, v := range u.Unverified {
		lines = append(lines, fmt.Sprintf("Unverified `%s`: %s", v.Finding, v.Reason))
	}
	lines = append(lines, u.Skipped...)
	lines = append(lines, u.Notes...)
	if u.BudgetExhausted {
		lines = append(lines, "The budget ran out; experts and verifications after that point did not run.")
	}
	var tokens int64
	for _, run := range r.Runs {
		tokens += run.Usage.Tokens
	}
	lines = append(lines, fmt.Sprintf("%d agent runs, %d tokens reported.", len(r.Runs), tokens))
	b.WriteString("\n## Uncertainty\n\n")
	for _, l := range lines {
		fmt.Fprintf(b, "- %s\n", l)
	}
}
