package workflow

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

const (
	maxParallel   = 4
	reproRuns     = 5
	fallbackPanel = 4
	expertPercent = 75
	judgeBatch    = 8
	quoteWords    = 3
	sameProblem   = 0.5
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

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityNote    Severity = "note"
)

type EvidenceKind string

const (
	EvidenceTest       EvidenceKind = "test"
	EvidenceAnalysis   EvidenceKind = "analysis"
	EvidenceInspection EvidenceKind = "inspection"
)

type Oracle string

const (
	OracleSpecified Oracle = "specified"
	OracleDerived   Oracle = "derived"
	OracleImplicit  Oracle = "implicit"
	OracleHuman     Oracle = "human"
)

type Kind string

const (
	KindFail   Kind = "fail"
	KindReview Kind = "review"
	KindOpen   Kind = "open"
)

type Status string

const (
	StatusUnverified    Status = "unverified"
	StatusNotReproduced Status = "not_reproduced"
	StatusReproduced    Status = "reproduced"
	StatusFixed         Status = "fix_verified"
	StatusPreExisting   Status = "pre_existing"
	StatusConfirmed     Status = "confirmed"
	StatusRejected      Status = "rejected"
	StatusResolved      Status = "resolved"
)

type TestStatus string

const (
	TestPassed TestStatus = "PASSED"
	TestFailed TestStatus = "FAILED"
	TestError  TestStatus = "ERROR"
)

type Label string

const (
	LabelIssue      Label = "issue"
	LabelQuestion   Label = "question"
	LabelSuggestion Label = "suggestion"
	LabelNitpick    Label = "nitpick"
)

type Role string

const (
	RoleCI         Role = "ci"
	RolePlanner    Role = "planner"
	RoleExpert     Role = "expert"
	RoleFix        Role = "fix"
	RoleRepro      Role = "repro"
	RoleJudge      Role = "judge"
	RoleClassifier Role = "classifier"
)

type PlanSource string

const (
	PlanFromPlanner PlanSource = "planner"
	PlanFromDefault PlanSource = "default"
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
	Files       []File   `json:"files"`
	Commits     []string `json:"commits"`
	Description string   `json:"description,omitempty"`
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
	Budget       Budget   `json:"budget"`
	Verification Mode     `json:"verification"`
	Experts      []string `json:"experts"`
	Previous     Result   `json:"previous,omitzero"`
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
	Solo     bool           `json:"solo,omitempty"`
	Blind    bool           `json:"blind,omitempty"`
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
	Test   Edit   `json:"test,omitzero"`
	Run    string `json:"run"`
	Fails  string `json:"fails"`
	Oracle Oracle `json:"oracle"`
	Quote  string `json:"quote,omitempty"`
	Source string `json:"source,omitempty"`
}

type Location struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

type Claim struct {
	Skill      string       `json:"skill"`
	Category   string       `json:"category,omitempty"`
	CWE        string       `json:"cwe,omitempty"`
	Path       string       `json:"path"`
	Line       int          `json:"line"`
	Related    []Location   `json:"related,omitempty"`
	Severity   Severity     `json:"severity"`
	Subject    string       `json:"subject"`
	Discussion string       `json:"discussion"`
	Scenario   string       `json:"scenario,omitempty"`
	Evidence   EvidenceKind `json:"evidence"`
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
	Tests []string `json:"tests"`
	Base  bool     `json:"base"`
}

type TestRun struct {
	Passed bool     `json:"passed"`
	Failed []string `json:"failed"`
	Output string   `json:"output"`
}

type Verification struct {
	Finding string       `json:"finding"`
	Status  Status       `json:"status"`
	Repro   *Repro       `json:"repro,omitempty"`
	Change  []TestStatus `json:"change,omitempty"`
	Base    TestStatus   `json:"base,omitempty"`
	Fixed   TestStatus   `json:"fixed,omitempty"`
	Fix     []Edit       `json:"fix,omitempty"`
	Reason  string       `json:"reason,omitempty"`
}

type Judgement struct {
	Finding string `json:"finding"`
	Valid   bool   `json:"valid"`
	Reason  string `json:"reason"`
}

type Merged struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Line      int       `json:"line"`
	Findings  []Finding `json:"findings"`
	Experts   []string  `json:"experts"`
	Kind      Kind      `json:"kind"`
	Group     int       `json:"group"`
	Objection string    `json:"objection,omitempty"`
}

type Comment struct {
	Finding     string     `json:"finding"`
	Label       Label      `json:"label"`
	Decorations []string   `json:"decorations"`
	Short       string     `json:"short"`
	Subject     string     `json:"subject"`
	Discussion  string     `json:"discussion"`
	Scenario    string     `json:"scenario,omitempty"`
	Path        string     `json:"path"`
	Line        int        `json:"line"`
	Related     []Location `json:"related,omitempty"`
	Experts     []string   `json:"experts"`
	Severity    Severity   `json:"severity"`
	Kind        Kind       `json:"kind"`
	Repro       *Repro     `json:"repro,omitempty"`
	Fix         []Edit     `json:"fix,omitempty"`
	Objection   string     `json:"objection,omitempty"`
}

type Usage struct {
	Model    string `json:"model"`
	Tokens   int64  `json:"tokens"`
	Cached   int64  `json:"cached"`
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
	Unverified      []Unverified `json:"unverified"`
	Skipped         []string     `json:"skipped"`
	Notes           []string     `json:"notes"`
	BudgetExhausted bool         `json:"budget_exhausted"`
}

type Result struct {
	Outcome       Outcome        `json:"outcome"`
	Reason        string         `json:"reason,omitempty"`
	Diagnosis     Diagnosis      `json:"diagnosis"`
	Triage        Triage         `json:"triage"`
	Regression    *TestRun       `json:"regression,omitempty"`
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
	Expert      Expert   `json:"expert"`
	Tier        Tier     `json:"tier"`
	Group       Group    `json:"group"`
	Plan        Plan     `json:"plan,omitzero"`
	Skills      []string `json:"skills"`
	Files       []File   `json:"files"`
	Commits     []string `json:"commits"`
	Description string   `json:"description,omitempty"`
	Regression  *TestRun `json:"regression,omitempty"`
	Rejected    []string `json:"rejected"`
	Earlier     []Claim  `json:"earlier,omitempty"`
	TestOne     string   `json:"-"`
}

type FixTask struct {
	Finding Finding `json:"finding"`
	Repro   Repro   `json:"repro"`
	Files   []File  `json:"files"`
}

type RetestTask struct {
	Finding Finding `json:"finding"`
	Command string  `json:"command"`
	Repro   *Repro  `json:"repro,omitempty"`
	Output  string  `json:"output,omitempty"`
}

type JudgeTask struct {
	Findings    []Finding      `json:"findings"`
	Tried       []Verification `json:"tried,omitempty"`
	Files       []File         `json:"files"`
	Commits     []string       `json:"commits"`
	Description string         `json:"description,omitempty"`
}

type Agents interface {
	Diagnose(ctx context.Context, t DiagnoseTask) (Diagnosis, Usage, error)
	Plan(ctx context.Context, t PlanTask) (pulls.Analysis, Usage, error)
	Review(ctx context.Context, t ReviewTask) (Report, Usage, error)
	Fix(ctx context.Context, t FixTask) ([]Edit, Usage, error)
	Retest(ctx context.Context, t RetestTask) (Repro, Usage, error)
	Judge(ctx context.Context, t JudgeTask) ([]Judgement, Usage, error)
}

type Classifier interface {
	Same(ctx context.Context, pairs [][2]Finding) ([]float64, Usage, error)
}

type Workspace interface {
	CI(ctx context.Context) (Check, error)
	Setup(ctx context.Context) error
	Read(ctx context.Context, path string) (string, error)
	Test(ctx context.Context, edits []Edit, t Target) (TestRun, error)
}

type Archive interface {
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Save(ctx context.Context, key string, data []byte) error
}

type Session struct {
	Agents     Agents
	Classifier Classifier
	Workspace  Workspace
	Archive    Archive
}

type Workflow struct {
	roster []Expert
	core   *pulls.Core
	log    *slog.Logger
}

type ciState struct {
	Check     Check     `json:"check"`
	Diagnosis Diagnosis `json:"diagnosis"`
	Runs      []Run     `json:"runs"`
}

type prepareState struct {
	Triage     Triage   `json:"triage"`
	SetupError string   `json:"setup_error,omitempty"`
	Skipped    []string `json:"skipped"`
	Runs       []Run    `json:"runs"`
}

type planState struct {
	Plan   Plan       `json:"plan"`
	Source PlanSource `json:"source"`
	Notes  []string   `json:"notes"`
	Runs   []Run      `json:"runs"`
}

type expertState struct {
	Findings []Claim  `json:"findings"`
	Unread   []string `json:"unread"`
	Dropped  []string `json:"dropped,omitempty"`
	Absent   string   `json:"absent,omitempty"`
	Runs     []Run    `json:"runs"`
}

type seat struct {
	pick   Pick
	groups []int
	state  expertState
	err    error
}

type verifyState struct {
	Verification Verification `json:"verification"`
	Runs         []Run        `json:"runs"`
}

type judgeState struct {
	Verifications []Verification `json:"verifications"`
	Runs          []Run          `json:"runs"`
}

type matchState struct {
	Root  map[string]string `json:"root"`
	Notes []string          `json:"notes"`
	Runs  []Run             `json:"runs"`
}

type suiteState struct {
	Change TestRun `json:"change"`
	Base   TestRun `json:"base"`
	Error  string  `json:"error,omitempty"`
}

type stored interface {
	ciState | prepareState | planState | expertState | verifyState | judgeState | matchState | suiteState
	spent() []Run
	open() bool
}

type reply interface {
	Diagnosis | pulls.Analysis | Report | []Edit | Repro | []Judgement | []float64
}

type task interface {
	DiagnoseTask | PlanTask | ReviewTask | FixTask | RetestTask | JudgeTask | [][2]Finding
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
	ErrOtherRequest   = errors.New("the archive holds the stages of another review request")
	ErrInvalidPatch   = errors.New("invalid patch")
)
