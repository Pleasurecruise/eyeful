package report

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func verification(c workflow.Comment, f workflow.Finding, old bool, reason string) string {
	var text string
	switch {
	case c.Objection != "":
		return fmt.Sprintf("A judge agent read the code and disputes this finding: %s It is kept for you to decide.", c.Objection)
	case c.Repro == nil && f.Evidence == workflow.EvidenceTest:
		return "eyeful did not see a reproduction test fail for it."
	case c.Repro == nil:
		return fmt.Sprintf("Not executed; the evidence is %s.", strings.ReplaceAll(string(f.Evidence), "_", " "))
	case old:
		return fmt.Sprintf("eyeful ran the reproduction test %s: it fails on the change and fails the same way on the base commit, so the problem was there before this change.", c.Repro.Run)
	case len(c.Fix) > 0:
		text = fmt.Sprintf("eyeful ran the reproduction test %s: it failed on the change in every run, not on the base commit, and passed with the suggested fix, and the suite stayed green.", c.Repro.Run)
	default:
		text = fmt.Sprintf("eyeful ran the reproduction test %s and it failed on the change in every run, not on the base commit; no fix was verified", c.Repro.Run)
		if reason != "" {
			text += ": " + reason
		}
		text += "."
	}
	text += fmt.Sprintf(" Expected behaviour from: %s", c.Repro.Oracle)
	if c.Repro.Oracle == workflow.OracleSpecified {
		text += fmt.Sprintf(" (%q)", c.Repro.Quote)
	}
	return text + "."
}

func Findings(r workflow.Result) (Output, error) {
	out := Output{
		Outcome: r.Outcome, Reason: r.Reason, Findings: []Finding{},
		Diagnosis: r.Diagnosis, NotReviewed: r.Triage.Excluded, Uncertainty: r.Uncertainty,
	}
	for _, c := range r.Comments {
		i := slices.IndexFunc(r.Findings, func(f workflow.Finding) bool { return f.ID == c.Finding })
		if i < 0 {
			return Output{}, fmt.Errorf("comment on unknown finding %q", c.Finding)
		}
		f := r.Findings[i]
		var reason string
		if v := slices.IndexFunc(r.Verifications, func(v workflow.Verification) bool { return v.Finding == f.ID }); v >= 0 {
			reason = r.Verifications[v].Reason
		}
		old := baselineState(r, f.ID) == "unchanged"
		item := Finding{
			File: c.Path, Line: c.Line, Related: c.Related, Severity: c.Severity, Kind: c.Kind,
			Category: strings.Join(slices.DeleteFunc(slices.Clone(c.Decorations), func(d string) bool { return slices.Contains(mergeDecorations, d) }), ", "), Summary: c.Subject,
			FailureScenario: c.Scenario, Why: c.Discussion,
			Verification: verification(c, f, old, reason),
			Label:        c.Label, Experts: c.Experts, Fix: c.Fix, Reproduced: c.Repro != nil && !old,
		}
		if c.Repro != nil && c.Repro.Test != (workflow.Edit{}) {
			item.Test = &c.Repro.Test
		}
		if item.Reproduced {
			out.Counts.Reproduced++
		}
		switch c.Kind {
		case workflow.KindFail:
			out.Counts.Fail++
		case workflow.KindReview:
			out.Counts.Review++
		case workflow.KindOpen:
			out.Counts.Open++
		}
		out.Findings = append(out.Findings, item)
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
