package report

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func Markdown(r workflow.Result) (string, error) {
	out, err := Findings(r)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# eyeful review\n\n%d findings: %d fail, %d review, %d open. Outcome `%s`.\n", len(out.Findings), out.Counts.Fail, out.Counts.Review, out.Counts.Open, out.Outcome)
	if out.Reason != "" {
		fmt.Fprintf(&b, "%s.\n", out.Reason)
	}
	change(&b, r.Plan)
	if out.Outcome == workflow.OutcomeCIFailed {
		b.WriteString("\n## CI failed\n\n")
		for _, f := range out.Diagnosis.Failures {
			fmt.Fprintf(&b, "- `%s` %s %s: %s\n  Fix: %s\n", f.Workflow, f.Job, f.Step, f.Cause, f.Fix)
		}
	}
	switch {
	case r.Regression == nil:
	case len(r.Regression.Failed) == 0:
		b.WriteString("\n## Tests the change breaks\n\nThe project's tests pass on the base commit and fail on the change. Its command does not name the tests that fail.\n")
	default:
		fmt.Fprintf(&b, "\n## Tests the change breaks\n\nThese pass on the base commit and fail on the change: `%s`.\n", strings.Join(r.Regression.Failed, "`, `"))
	}
	resolved(&b, r)
	if out.Counts.Fail == 0 && out.Outcome != workflow.OutcomeCIFailed {
		b.WriteString("\nNo blocking issues.\n")
	}
	findings(&b, out.Findings)
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

func Summary(r workflow.Result) (string, error) {
	out, err := Findings(r)
	if err != nil {
		return "", err
	}
	var posted, rest []Finding
	for _, f := range out.Findings {
		if Posted(f.Label, f.Kind) {
			posted = append(posted, f)
		} else {
			rest = append(rest, f)
		}
	}
	var b strings.Builder
	switch len(posted) {
	case 0:
		b.WriteString("### ✅ No issues found\n")
	case 1:
		b.WriteString("### ⚠️ 1 issue found\n")
	default:
		fmt.Fprintf(&b, "### ⚠️ %d issues found\n", len(posted))
	}
	fmt.Fprintf(&b, "\nOutcome `%s`, %d findings: %d reproduced, %d to review, %d disputed.", out.Outcome, len(out.Findings), out.Counts.Fail, out.Counts.Review, out.Counts.Open)
	if out.Counts.Fail == 0 {
		b.WriteString(" None was reproduced by running a test.")
	}
	if out.Reason != "" {
		fmt.Fprintf(&b, " %s.", out.Reason)
	}
	b.WriteString("\n")
	for _, f := range out.Diagnosis.Failures {
		fmt.Fprintf(&b, "\nCI failed: `%s` %s %s: %s Fix: %s\n", f.Workflow, f.Job, f.Step, f.Cause, f.Fix)
	}
	switch {
	case r.Regression == nil:
	case len(r.Regression.Failed) == 0:
		b.WriteString("\nThe project's tests pass on the base commit and fail on the change.\n")
	default:
		fmt.Fprintf(&b, "\nThe change breaks `%s`.\n", strings.Join(r.Regression.Failed, "`, `"))
	}
	if len(posted) > 0 {
		b.WriteString("\n| Severity | File:Line | Issue |\n| --- | --- | --- |\n")
		for _, f := range posted {
			fmt.Fprintf(&b, "| %s | `%s:%d` | %s |\n", markers[f.Severity], f.File, f.Line, f.ShortSummary)
		}
	}
	if len(rest) > 0 {
		fmt.Fprintf(&b, "\n<details><summary>Not posted (%d)</summary>\n\n", len(rest))
		for _, f := range rest {
			fmt.Fprintf(&b, "- %s · `%s:%d` %s\n", f.Label, f.File, f.Line, f.ShortSummary)
		}
		b.WriteString("\n</details>\n")
	}
	return b.String(), nil
}

func Posted(label workflow.Label, kind workflow.Kind) bool {
	return label == workflow.LabelIssue && kind != workflow.KindOpen
}

func resolved(b *strings.Builder, r workflow.Result) {
	first := true
	for _, v := range r.Verifications {
		i := slices.IndexFunc(r.Findings, func(f workflow.Finding) bool { return f.ID == v.Finding })
		if i < 0 || v.Status != workflow.StatusResolved {
			continue
		}
		if first {
			b.WriteString("\n## Resolved since the last review\n\nEach of these failed its reproduction in the last review and passes it on this change.\n\n")
			first = false
		}
		fmt.Fprintf(b, "- `%s:%d` %s (`%s`)\n", r.Findings[i].Path, r.Findings[i].Line, r.Findings[i].Subject, v.Repro.Run)
	}
}

func findings(b *strings.Builder, all []Finding) {
	var shown, disputed, nitpicks []Finding
	for _, f := range all {
		switch {
		case f.Label == workflow.LabelNitpick:
			nitpicks = append(nitpicks, f)
		case f.Kind == workflow.KindOpen:
			disputed = append(disputed, f)
		default:
			shown = append(shown, f)
		}
	}
	if len(shown) > 0 {
		b.WriteString("\n| Severity | File:Line | Issue |\n| --- | --- | --- |\n")
		for _, f := range shown {
			fmt.Fprintf(b, "| %s | `%s:%d` | %s |\n", markers[f.Severity], f.File, f.Line, f.ShortSummary)
		}
	}
	for _, f := range shown {
		finding(b, f)
	}
	if len(disputed) > 0 {
		fmt.Fprintf(b, "\n<details><summary>Disputed (%d)</summary>\n\nThe judge agent read the code and does not think these hold. They are kept for you to decide.\n\n", len(disputed))
		for _, f := range disputed {
			fmt.Fprintf(b, "- `%s:%d` %s %s (%s)\n", f.File, f.Line, f.Summary, f.Verification, strings.Join(f.Experts, ", "))
		}
		b.WriteString("\n</details>\n")
	}
	if len(nitpicks) > 0 {
		fmt.Fprintf(b, "\n<details><summary>Nitpicks (%d)</summary>\n\n", len(nitpicks))
		for _, f := range nitpicks {
			fmt.Fprintf(b, "- `%s:%d` %s %s (%s)\n", f.File, f.Line, f.Summary, strings.Join(strings.Fields(f.Why), " "), strings.Join(f.Experts, ", "))
		}
		b.WriteString("\n</details>\n")
	}
}

func finding(b *strings.Builder, f Finding) {
	fmt.Fprintf(b, "\n## %s · %s · %s\n\n`%s:%d` %s\n", markers[f.Severity], f.Category, f.Kind, f.File, f.Line, f.Summary)
	if len(f.Related) > 0 {
		places := make([]string, len(f.Related))
		for i, r := range f.Related {
			places[i] = fmt.Sprintf("`%s:%d`", r.Path, r.Line)
		}
		fmt.Fprintf(b, "\nAlso at %s.\n", strings.Join(places, ", "))
	}
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
		lines = append(lines, fmt.Sprintf("Expert `%s` is absent: %s", a.Expert, a.Reason))
	}
	for _, v := range u.Unverified {
		lines = append(lines, fmt.Sprintf("Unverified `%s`: %s", v.Finding, v.Reason))
	}
	lines = append(lines, u.Skipped...)
	lines = append(lines, u.Notes...)
	if u.BudgetExhausted {
		lines = append(lines, "The budget ran out; experts and verifications after that point did not run.")
	}
	var tokens, cached int64
	for _, run := range r.Runs {
		tokens += run.Usage.Tokens
		cached += run.Usage.Cached
	}
	lines = append(lines, fmt.Sprintf("%d agent runs, %d tokens reported, and %d more read from the cache.", len(r.Runs), tokens, cached))
	b.WriteString("\n## Uncertainty\n\n")
	for _, l := range lines {
		fmt.Fprintf(b, "- %s\n", l)
	}
}

func change(b *strings.Builder, p workflow.Plan) {
	if p.Summary == "" {
		return
	}
	fmt.Fprintf(b, "\n## What this change does\n\n%s\n\n| Group | Category | Why | Files |\n| --- | --- | --- | --- |\n", p.Summary)
	groups := slices.Clone(p.Groups)
	slices.SortStableFunc(groups, func(a, b workflow.Group) int {
		switch {
		case a.Core == b.Core:
			return 0
		case a.Core:
			return -1
		}
		return 1
	})
	for i, g := range groups {
		name := strconv.Itoa(i + 1)
		if g.Core {
			name += " (core)"
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s |\n", name, g.Category, g.Summary, "`"+strings.Join(g.Files, "`, `")+"`")
	}
}
