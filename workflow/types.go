package workflow

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

const maxParallel = 4

type Level string

const (
	LevelQuick    Level = "quick"
	LevelStandard Level = "standard"
	LevelDeep     Level = "deep"
)

type Tier string

const (
	TierCheap  Tier = "cheap"
	TierMid    Tier = "mid"
	TierStrong Tier = "strong"
)

type Mode string

const (
	ModeExecution Mode = "execution"
	ModeReadOnly  Mode = "read_only"
	ModeNone      Mode = "none"
)

type Outcome string

const (
	OutcomeComplete Outcome = "complete"
	OutcomeNone     Outcome = "none"
	OutcomeCIFailed Outcome = "ci_failed"
	OutcomePartial  Outcome = "partial"
)

type Class string

const (
	ClassCode      Class = "code"
	ClassTest      Class = "test"
	ClassDocs      Class = "docs"
	ClassLock      Class = "lock"
	ClassGenerated Class = "generated"
	ClassVendored  Class = "vendored"
	ClassBinary    Class = "binary"
	ClassSkipped   Class = "skipped"
	ClassOversized Class = "oversized"
)

type Tool string

const (
	ToolLint            Tool = "lint"
	ToolTypecheck       Tool = "typecheck"
	ToolSecretScan      Tool = "secret_scan"
	ToolDependencyAudit Tool = "dependency_audit"
	ToolOpenAPIDiff     Tool = "openapi_diff"
	ToolComplexity      Tool = "complexity"
	ToolCoverageDiff    Tool = "coverage_diff"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

type EvidenceKind string

const (
	EvidenceReproTest   EvidenceKind = "repro_test"
	EvidenceCVE         EvidenceKind = "cve"
	EvidenceAPIBreak    EvidenceKind = "api_break"
	EvidenceCoverage    EvidenceKind = "coverage"
	EvidenceTool        EvidenceKind = "tool"
	EvidenceE2E         EvidenceKind = "e2e"
	EvidenceTriggerPath EvidenceKind = "trigger_path"
	EvidenceArgument    EvidenceKind = "argument"
)

type Oracle string

const (
	OracleImplicit Oracle = "implicit"
	OracleExisting Oracle = "existing"
	OracleSpec     Oracle = "spec"
	OracleBase     Oracle = "base"
	OracleAgent    Oracle = "agent"
)

type Strength int

const (
	StrengthWeak Strength = iota + 1
	StrengthMedium
	StrengthStrong
)

type Status string

const (
	StatusUnverified    Status = "unverified"
	StatusNotReproduced Status = "not_reproduced"
	StatusReproduced    Status = "reproduced"
	StatusFixed         Status = "fix_verified"
	StatusConfirmed     Status = "confirmed"
	StatusRejected      Status = "rejected"
)

type Label string

const (
	LabelIssue      Label = "issue"
	LabelTodo       Label = "todo"
	LabelChore      Label = "chore"
	LabelSuggestion Label = "suggestion"
	LabelQuestion   Label = "question"
	LabelThought    Label = "thought"
	LabelNote       Label = "note"
	LabelNitpick    Label = "nitpick"
	LabelPraise     Label = "praise"
)

type Role string

const (
	RoleCI         Role = "ci"
	RolePlanner    Role = "planner"
	RoleExpert     Role = "expert"
	RoleFix        Role = "fix"
	RoleJudge      Role = "judge"
	RoleSummarizer Role = "summarizer"
)

type PlanSource string

const (
	PlanFromPlanner PlanSource = "planner"
	PlanFromDefault PlanSource = "default"
	PlanFromRules   PlanSource = "rules"
	PlanFromRequest PlanSource = "request"
)

const SkillOther = "other"

type File struct {
	Path    string `json:"path"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
	Binary  bool   `json:"binary"`
	Diff    string `json:"diff"`
}

type Change struct {
	Files   []File   `json:"files"`
	Commits []string `json:"commits"`
}

type Project struct {
	Skip    []string `json:"skip"`
	Risk    []string `json:"risk"`
	TestOne string   `json:"test_one"`
}

type Budget struct {
	Tokens   int64 `json:"tokens"`
	MicroUSD int64 `json:"micro_usd"`
}

type Request struct {
	Change       Change   `json:"change"`
	Project      Project  `json:"project"`
	Level        Level    `json:"level"`
	Budget       Budget   `json:"budget"`
	Verification Mode     `json:"verification"`
	Experts      []string `json:"experts"`
}

type Skill struct {
	Name  string   `json:"name"`
	Files []string `json:"files,omitempty"`
}

type Expert struct {
	Name     string         `json:"name"`
	Area     string         `json:"area"`
	Tier     Tier           `json:"tier"`
	Skills   []Skill        `json:"skills"`
	Evidence []EvidenceKind `json:"evidence"`
}

type Entry struct {
	Path    string `json:"path"`
	Class   Class  `json:"class"`
	Added   int    `json:"added"`
	Deleted int    `json:"deleted"`
}

type Triage struct {
	Reviewed []Entry  `json:"reviewed"`
	Excluded []Entry  `json:"excluded"`
	Risk     []string `json:"risk"`
}

type Hit struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

type ToolResult struct {
	Tool Tool  `json:"tool"`
	Hits []Hit `json:"hits"`
}

type Pick struct {
	Name string `json:"name"`
	Why  string `json:"why"`
}

type Scope struct {
	Name  string   `json:"name"`
	Files []string `json:"files"`
	Lines int      `json:"lines"`
}

type Group struct {
	Scope    int      `json:"scope"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Core     bool     `json:"core"`
	Files    []string `json:"files"`
	Experts  []Pick   `json:"experts"`
}

type Plan struct {
	Summary    string  `json:"summary"`
	Groups     []Group `json:"groups"`
	Skipped    []Pick  `json:"skipped"`
	Confidence float64 `json:"confidence"`
}

type Edit struct {
	Path string `json:"path"`
	Old  string `json:"old"`
	New  string `json:"new"`
}

type Repro struct {
	Test   Edit   `json:"test"`
	Run    string `json:"run"`
	Oracle Oracle `json:"oracle"`
	Quote  string `json:"quote,omitempty"`
}

type Claim struct {
	Skill      string       `json:"skill"`
	Category   string       `json:"category,omitempty"`
	CWE        string       `json:"cwe,omitempty"`
	Path       string       `json:"path"`
	Line       int          `json:"line"`
	Severity   Severity     `json:"severity"`
	Subject    string       `json:"subject"`
	Discussion string       `json:"discussion"`
	Scenario   string       `json:"scenario,omitempty"`
	Evidence   EvidenceKind `json:"evidence"`
	Repro      *Repro       `json:"repro,omitempty"`
}

type Finding struct {
	ID     string `json:"id"`
	Expert string `json:"expert"`
	Group  int    `json:"group"`
	Claim
}

type Report struct {
	Findings []Claim  `json:"findings"`
	Loaded   []string `json:"-"`
}

type Failure struct {
	Workflow string `json:"workflow"`
	Job      string `json:"job"`
	Step     string `json:"step"`
	Cause    string `json:"cause"`
	Fix      string `json:"fix"`
}

type Diagnosis struct {
	Failures []Failure `json:"failures"`
}

type Check struct {
	Passed bool   `json:"passed"`
	Log    string `json:"log"`
}

type Target struct {
	Tests   []string `json:"tests"`
	Touched []string `json:"touched"`
	Full    bool     `json:"full"`
}

type TestRun struct {
	Passed bool     `json:"passed"`
	Failed []string `json:"failed"`
	Output string   `json:"output"`
}

type Verification struct {
	Finding string `json:"finding"`
	Status  Status `json:"status"`
	Fix     []Edit `json:"fix,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

type Judgement struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason"`
}

type Merged struct {
	ID       string    `json:"id"`
	Path     string    `json:"path"`
	Line     int       `json:"line"`
	Findings []Finding `json:"findings"`
	Experts  []string  `json:"experts"`
	Strength Strength  `json:"strength"`
	Group    int       `json:"group"`
}

type Draft struct {
	Finding    string `json:"finding"`
	Label      Label  `json:"label"`
	Short      string `json:"short"`
	Subject    string `json:"subject"`
	Discussion string `json:"discussion"`
}

type Comment struct {
	Finding     string   `json:"finding"`
	Label       Label    `json:"label"`
	Decorations []string `json:"decorations"`
	Short       string   `json:"short"`
	Subject     string   `json:"subject"`
	Discussion  string   `json:"discussion"`
	Scenario    string   `json:"scenario,omitempty"`
	Path        string   `json:"path"`
	Line        int      `json:"line"`
	Experts     []string `json:"experts"`
	Strength    Strength `json:"strength"`
	Test        *Edit    `json:"test,omitempty"`
	Fix         []Edit   `json:"fix,omitempty"`
}

type Usage struct {
	Model    string `json:"model"`
	Tokens   int64  `json:"tokens"`
	MicroUSD int64  `json:"micro_usd"`
}

type Run struct {
	Role   Role   `json:"role"`
	Expert string `json:"expert,omitempty"`
	Usage  Usage  `json:"usage"`
	Error  string `json:"error,omitempty"`
}

type Absence struct {
	Expert string `json:"expert"`
	Group  int    `json:"group"`
	Reason string `json:"reason"`
}

type Unread struct {
	Expert string `json:"expert"`
	Group  int    `json:"group"`
	Skill  string `json:"skill"`
}

type Unverified struct {
	Finding string `json:"finding"`
	Reason  string `json:"reason"`
}

type Uncertainty struct {
	Absent          []Absence    `json:"absent"`
	Unread          []Unread     `json:"unread"`
	Dismissed       []Finding    `json:"dismissed"`
	Unverified      []Unverified `json:"unverified"`
	Skipped         []string     `json:"skipped"`
	Notes           []string     `json:"notes"`
	BudgetExhausted bool         `json:"budget_exhausted"`
}

type Result struct {
	Outcome       Outcome        `json:"outcome"`
	Reason        string         `json:"reason,omitempty"`
	Level         Level          `json:"level"`
	Diagnosis     Diagnosis      `json:"diagnosis"`
	Triage        Triage         `json:"triage"`
	Tools         []ToolResult   `json:"tools"`
	Scopes        []Scope        `json:"scopes"`
	Plan          Plan           `json:"plan"`
	PlanSource    PlanSource     `json:"plan_source,omitempty"`
	Findings      []Finding      `json:"findings"`
	Verifications []Verification `json:"verifications"`
	Comments      []Comment      `json:"comments"`
	Uncertainty   Uncertainty    `json:"uncertainty"`
	Runs          []Run          `json:"runs"`
}

type DiagnoseTask struct {
	Log string `json:"log"`
}

type PlanTask struct {
	Diff     pulls.DiffsPayload `json:"-"`
	Rejected []string           `json:"rejected"`
}

type ReviewTask struct {
	Expert   Expert       `json:"expert"`
	Level    Level        `json:"level"`
	Tier     Tier         `json:"tier"`
	Group    Group        `json:"group"`
	Plan     Plan         `json:"plan"`
	Skills   []string     `json:"skills"`
	Files    []File       `json:"files"`
	Tools    []ToolResult `json:"tools"`
	Rejected []string     `json:"rejected"`
	TestOne  string       `json:"-"`
}

type FixTask struct {
	Finding Finding `json:"finding"`
	Files   []File  `json:"files"`
}

type JudgeTask struct {
	Finding Finding `json:"finding"`
	Files   []File  `json:"files"`
}

type SummaryTask struct {
	Findings []Merged `json:"findings"`
}

type Agents interface {
	Diagnose(ctx context.Context, t DiagnoseTask) (Diagnosis, Usage, error)
	Plan(ctx context.Context, t PlanTask) (pulls.Analysis, Usage, error)
	Review(ctx context.Context, t ReviewTask) (Report, Usage, error)
	Fix(ctx context.Context, t FixTask) ([]Edit, Usage, error)
	Judge(ctx context.Context, t JudgeTask) (Judgement, Usage, error)
	Summarize(ctx context.Context, t SummaryTask) ([]Draft, Usage, error)
}

type Workspace interface {
	CI(ctx context.Context) (Check, error)
	Setup(ctx context.Context) error
	Tools(ctx context.Context, files []string) ([]ToolResult, error)
	Test(ctx context.Context, edits []Edit, t Target) (TestRun, error)
}

type Archive interface {
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Save(ctx context.Context, key string, data []byte) error
}

type Session struct {
	Agents    Agents
	Workspace Workspace
	Archive   Archive
}

type Workflow struct {
	roster []Expert
	core   *pulls.Core
	log    *slog.Logger
}

type limits struct {
	experts int
	nits    bool
	planner bool
	verify  bool
	full    bool
	tier    Tier
}

type ciState struct {
	Check     Check     `json:"check"`
	Diagnosis Diagnosis `json:"diagnosis"`
	Runs      []Run     `json:"runs"`
}

type prepareState struct {
	Triage     Triage       `json:"triage"`
	SetupError string       `json:"setup_error,omitempty"`
	Tools      []ToolResult `json:"tools"`
	Skipped    []string     `json:"skipped"`
	Runs       []Run        `json:"runs"`
}

type planState struct {
	Level  Level      `json:"level"`
	Plan   Plan       `json:"plan"`
	Source PlanSource `json:"source"`
	Notes  []string   `json:"notes"`
	Runs   []Run      `json:"runs"`
}

type expertState struct {
	Findings []Claim  `json:"findings"`
	Unread   []string `json:"unread"`
	Absent   string   `json:"absent,omitempty"`
	Runs     []Run    `json:"runs"`
}

type verifyState struct {
	Verification Verification `json:"verification"`
	Runs         []Run        `json:"runs"`
}

type baselineState struct {
	Run   TestRun `json:"run"`
	Error string  `json:"error,omitempty"`
}

type summaryState struct {
	Comments []Comment `json:"comments"`
	Fallback string    `json:"fallback,omitempty"`
	Runs     []Run     `json:"runs"`
}

type stored interface {
	ciState | prepareState | planState | expertState | verifyState | baselineState | summaryState
	spent() []Run
}

type reply interface {
	Diagnosis | pulls.Analysis | Report | []Edit | Judgement | []Draft
}

type task interface {
	DiagnoseTask | PlanTask | ReviewTask | FixTask | JudgeTask | SummaryTask
}

type ledger struct {
	mu        sync.Mutex
	budget    Budget
	spent     Budget
	runs      []Run
	exhausted bool
	parent    *ledger
}

var (
	ErrInvalidRoster  = errors.New("invalid expert roster")
	ErrInvalidRequest = errors.New("invalid review request")
	ErrInvalidPatch   = errors.New("invalid patch")
)
