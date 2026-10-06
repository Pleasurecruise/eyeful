package report

import "github.com/Pleasurecruise/eyeful/workflow"

type Severity string

const (
	SeverityImportant Severity = "important"
	SeverityNit       Severity = "nit"
)

type Verdict string

const (
	VerdictConfirmed Verdict = "CONFIRMED"
	VerdictPlausible Verdict = "PLAUSIBLE"
)

type Finding struct {
	File            string          `json:"file"`
	Line            int             `json:"line"`
	Severity        Severity        `json:"severity"`
	Verdict         Verdict         `json:"verdict"`
	Category        string          `json:"category"`
	ShortSummary    string          `json:"short_summary"`
	Summary         string          `json:"summary"`
	FailureScenario string          `json:"failure_scenario,omitempty"`
	Why             string          `json:"why"`
	Verification    string          `json:"verification"`
	Label           workflow.Label  `json:"label"`
	Experts         []string        `json:"experts"`
	Test            *workflow.Edit  `json:"test,omitempty"`
	Fix             []workflow.Edit `json:"fix,omitempty"`
}

type Counts struct {
	Important int `json:"important"`
	Nit       int `json:"nit"`
}

type Output struct {
	Outcome     workflow.Outcome     `json:"outcome"`
	Reason      string               `json:"reason,omitempty"`
	Level       workflow.Level       `json:"level"`
	Counts      Counts               `json:"counts"`
	Findings    []Finding            `json:"findings"`
	Diagnosis   workflow.Diagnosis   `json:"diagnosis"`
	NotReviewed []workflow.Entry     `json:"not_reviewed"`
	Uncertainty workflow.Uncertainty `json:"uncertainty"`
}

var mergeDecorations = []string{"blocking", "non-blocking", "if-minor"}

var markers = map[Severity]string{SeverityImportant: "🔴 Important", SeverityNit: "🟡 Nit"}
