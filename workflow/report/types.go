package report

import "github.com/Pleasurecruise/eyeful/workflow"

type Finding struct {
	File            string              `json:"file"`
	Line            int                 `json:"line"`
	Related         []workflow.Location `json:"related,omitempty"`
	Severity        workflow.Severity   `json:"severity"`
	Kind            workflow.Kind       `json:"kind"`
	Category        string              `json:"category"`
	Summary         string              `json:"summary"`
	FailureScenario string              `json:"failure_scenario,omitempty"`
	Why             string              `json:"why"`
	Verification    string              `json:"verification"`
	Label           workflow.Label      `json:"label"`
	Reproduced      bool                `json:"reproduced"`
	Experts         []string            `json:"experts"`
	Test            *workflow.Edit      `json:"test,omitempty"`
	Fix             []workflow.Edit     `json:"fix,omitempty"`
}

type Counts struct {
	Fail       int `json:"fail"`
	Review     int `json:"review"`
	Open       int `json:"open"`
	Reproduced int `json:"reproduced"`
}

type Note struct {
	Path string
	Line int
	Body string
}

type Output struct {
	Outcome     workflow.Outcome     `json:"outcome"`
	Reason      string               `json:"reason,omitempty"`
	Counts      Counts               `json:"counts"`
	Findings    []Finding            `json:"findings"`
	Diagnosis   workflow.Diagnosis   `json:"diagnosis"`
	NotReviewed []workflow.Entry     `json:"not_reviewed"`
	Uncertainty workflow.Uncertainty `json:"uncertainty"`
}

var mergeDecorations = []string{"blocking", "non-blocking"}

var markers = map[workflow.Severity]string{workflow.SeverityError: "🔴 error", workflow.SeverityWarning: "🟠 warning", workflow.SeverityNote: "🟡 note"}
