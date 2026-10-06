package report

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/owenrumney/go-sarif/v3/pkg/report/v210/sarif"

	"github.com/Pleasurecruise/eyeful/workflow"
)

// TODO(report): write fixes as SARIF artifactChanges once edits carry line regions.
func SARIF(r workflow.Result) ([]byte, error) {
	run := sarif.NewRunWithInformationURI("eyeful", "https://github.com/Pleasurecruise/eyeful")
	run.WithResults([]*sarif.Result{})
	for _, c := range r.Comments {
		i := slices.IndexFunc(r.Findings, func(f workflow.Finding) bool { return f.ID == c.Finding })
		if i < 0 {
			return nil, fmt.Errorf("comment on unknown finding %q", c.Finding)
		}
		f := r.Findings[i]
		level := "note"
		switch {
		case slices.Contains(c.Decorations, "blocking"):
			level = "error"
		case c.Label == workflow.LabelIssue:
			level = "warning"
		}
		kind := "review"
		if c.Test != nil {
			kind = "fail"
		}
		rule := f.Skill
		if rule == workflow.SkillOther && f.Category != "" {
			rule = f.Category
		}
		props := sarif.NewPropertyBag().
			Add("expert", f.Expert).
			Add("evidence", f.Evidence).
			Add("label", c.Label).
			Add("decorations", c.Decorations).
			Add("corroboratedBy", slices.DeleteFunc(slices.Clone(c.Experts), func(e string) bool { return e == f.Expert }))
		if f.Repro != nil {
			props.Add("oracle", f.Repro.Oracle)
		}
		if f.CWE != "" {
			props.Add("cwe", f.CWE)
		}
		if c.Test != nil {
			props.Add("test", c.Test)
		}
		if len(c.Fix) > 0 {
			props.Add("fix", c.Fix)
		}
		location := sarif.NewLocationWithPhysicalLocation(sarif.NewPhysicalLocation().
			WithArtifactLocation(sarif.NewSimpleArtifactLocation(c.Path)).
			WithRegion(sarif.NewRegion().WithStartLine(c.Line)))
		run.AddResult(sarif.NewRuleResult(fmt.Sprintf("eyeful/%s/%s", f.Expert, rule)).
			WithLevel(level).
			WithKind(kind).
			WithMessage(sarif.NewTextMessage(c.Subject + "\n\n" + c.Discussion)).
			AddLocation(location).
			WithProperties(props))
	}
	log := sarif.NewReport().AddRun(run)
	if err := log.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sarif: %w", err)
	}
	var buf bytes.Buffer
	if err := log.PrettyWrite(&buf); err != nil {
		return nil, fmt.Errorf("encode sarif: %w", err)
	}
	return buf.Bytes(), nil
}
