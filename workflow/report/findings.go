package report

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func verification(c workflow.Comment, f workflow.Finding) string {
	switch {
	case c.Test != nil && len(c.Fix) > 0:
		return fmt.Sprintf("eyeful ran the reproduction test %s: it failed on the change and passed with the suggested fix, and the suite stayed green.", c.Test.Path)
	case c.Test != nil:
		return fmt.Sprintf("eyeful ran the reproduction test %s and it failed on the change; no fix was verified.", c.Test.Path)
	case f.Repro != nil:
		return "A reproduction test was proposed but eyeful did not see it fail."
	}
	return fmt.Sprintf("Not executed; the evidence is %s.", strings.ReplaceAll(string(f.Evidence), "_", " "))
}

func Findings(r workflow.Result) (Output, error) {
	out := Output{
		Outcome: r.Outcome, Reason: r.Reason, Level: r.Level, Findings: []Finding{},
		Diagnosis: r.Diagnosis, NotReviewed: r.Triage.Excluded, Uncertainty: r.Uncertainty,
	}
	for _, c := range r.Comments {
		if c.Label == workflow.LabelPraise {
			continue
		}
		i := slices.IndexFunc(r.Findings, func(f workflow.Finding) bool { return f.ID == c.Finding })
		if i < 0 {
			return Output{}, fmt.Errorf("comment on unknown finding %q", c.Finding)
		}
		f := r.Findings[i]
		item := Finding{
			File: c.Path, Line: c.Line, Severity: SeverityNit, Verdict: VerdictPlausible,
			Category: strings.Join(slices.DeleteFunc(slices.Clone(c.Decorations), func(d string) bool { return slices.Contains(mergeDecorations, d) }), ", "), ShortSummary: c.Short, Summary: c.Subject,
			FailureScenario: c.Scenario, Why: c.Discussion, Verification: verification(c, f),
			Label: c.Label, Experts: c.Experts, Test: c.Test, Fix: c.Fix,
		}
		if slices.Contains(c.Decorations, "blocking") {
			item.Severity = SeverityImportant
		}
		if c.Strength == workflow.StrengthStrong {
			item.Verdict = VerdictConfirmed
		}
		if f.Repro != nil && f.Repro.Oracle != "" {
			item.Verification += fmt.Sprintf(" Expected behaviour from: %s", f.Repro.Oracle)
			if f.Repro.Quote != "" {
				item.Verification += fmt.Sprintf(" (%q)", f.Repro.Quote)
			}
			item.Verification += "."
		}
		out.Findings = append(out.Findings, item)
	}
	slices.SortStableFunc(out.Findings, func(a, b Finding) int {
		if a.Severity == b.Severity {
			return 0
		}
		if a.Severity == SeverityImportant {
			return -1
		}
		return 1
	})
	for _, f := range out.Findings {
		if f.Severity == SeverityImportant {
			out.Counts.Important++
		} else {
			out.Counts.Nit++
		}
	}
	return out, nil
}

func JSON(r workflow.Result) ([]byte, error) {
	out, err := Findings(r)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode findings: %w", err)
	}
	return append(data, '\n'), nil
}
