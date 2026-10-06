package workflow_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Pleasurecruise/eyeful/workflow"
)

type agents struct {
	mu        sync.Mutex
	calls     map[workflow.Role]int
	tiers     []workflow.Tier
	diagnose  func() (workflow.Diagnosis, error)
	plan      func(t workflow.PlanTask) (workflow.Plan, error)
	review    func(t workflow.ReviewTask) (workflow.Report, error)
	fix       func(t workflow.FixTask) ([]workflow.Edit, error)
	judge     func(t workflow.JudgeTask) (workflow.Judgement, error)
	summarize func(t workflow.SummaryTask) ([]workflow.Draft, error)
}

var usage = workflow.Usage{Model: "test/model", Tokens: 100, MicroUSD: 10}

func (a *agents) count(r workflow.Role) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.calls == nil {
		a.calls = map[workflow.Role]int{}
	}
	a.calls[r]++
}

func (a *agents) n(r workflow.Role) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.calls[r]
}

func (a *agents) Diagnose(_ context.Context, _ workflow.DiagnoseTask) (workflow.Diagnosis, workflow.Usage, error) {
	a.count(workflow.RoleCI)
	d, err := a.diagnose()
	return d, usage, err
}

func (a *agents) Plan(_ context.Context, t workflow.PlanTask) (workflow.Plan, workflow.Usage, error) {
	a.count(workflow.RolePlanner)
	p, err := a.plan(t)
	return p, usage, err
}

func (a *agents) Review(_ context.Context, t workflow.ReviewTask) (workflow.Report, workflow.Usage, error) {
	a.count(workflow.RoleExpert)
	a.mu.Lock()
	a.tiers = append(a.tiers, t.Tier)
	a.mu.Unlock()
	r, err := a.review(t)
	return r, usage, err
}

func (a *agents) Fix(_ context.Context, t workflow.FixTask) ([]workflow.Edit, workflow.Usage, error) {
	a.count(workflow.RoleFix)
	e, err := a.fix(t)
	return e, usage, err
}

func (a *agents) Judge(_ context.Context, t workflow.JudgeTask) (workflow.Judgement, workflow.Usage, error) {
	a.count(workflow.RoleJudge)
	j, err := a.judge(t)
	return j, usage, err
}

func (a *agents) Summarize(_ context.Context, t workflow.SummaryTask) ([]workflow.Draft, workflow.Usage, error) {
	a.count(workflow.RoleSummarizer)
	d, err := a.summarize(t)
	return d, usage, err
}

type workspace struct {
	ci       workflow.Check
	setup    error
	tools    []workflow.ToolResult
	broken   []string
	baseline []string
	tests    int
}

func (w *workspace) CI(context.Context) (workflow.Check, error) { return w.ci, nil }
func (w *workspace) Setup(context.Context) error                { return w.setup }
func (w *workspace) Tools(context.Context, []string) ([]workflow.ToolResult, error) {
	return w.tools, nil
}

func (w *workspace) Test(_ context.Context, edits []workflow.Edit, t workflow.Target) (workflow.TestRun, error) {
	w.tests++
	fixed := slices.ContainsFunc(edits, func(e workflow.Edit) bool { return e.Old == "bug" })
	if len(t.Tests) == 0 {
		failed := w.baseline
		if fixed {
			failed = slices.Concat(w.baseline, w.broken)
		}
		return workflow.TestRun{Passed: len(failed) == 0, Failed: failed}, nil
	}
	reproduces := slices.ContainsFunc(edits, func(e workflow.Edit) bool { return strings.Contains(e.New, "real") })
	return workflow.TestRun{Passed: fixed || !reproduces}, nil
}

type archive struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (a *archive) Load(_ context.Context, key string) ([]byte, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	d, ok := a.data[key]
	return d, ok, nil
}

func (a *archive) Save(_ context.Context, key string, data []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.data == nil {
		a.data = map[string][]byte{}
	}
	a.data[key] = data
	return nil
}

var roster = []workflow.Expert{
	{Name: "correctness", Area: "correctness", Tier: workflow.TierStrong, Skills: []workflow.Skill{{Name: "code-review"}, {Name: "golang-concurrency", Files: []string{"**/*.go"}}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceReproTest, workflow.EvidenceArgument}},
	{Name: "security", Area: "security", Tier: workflow.TierStrong, Skills: []workflow.Skill{{Name: "security-review"}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceReproTest, workflow.EvidenceCVE, workflow.EvidenceTriggerPath}},
}

func change() workflow.Change {
	return workflow.Change{
		Files: []workflow.File{
			{Path: "app/session.py", Added: 3, Deleted: 1, Diff: "+bug\n"},
			{Path: "app/login.py", Added: 5, Deleted: 0, Diff: "+x\n"},
			{Path: "uv.lock", Added: 40, Deleted: 2},
		},
		Commits: []string{"fix session expiry"},
	}
}

func request() workflow.Request {
	return workflow.Request{
		Change: change(), Level: workflow.LevelStandard, Verification: workflow.ModeExecution,
		Budget: workflow.Budget{Tokens: 1_000_000, MicroUSD: 1_000_000},
	}
}

func goodPlan(workflow.PlanTask) (workflow.Plan, error) {
	return workflow.Plan{
		Summary: "Sessions expire at the exact expiry second, and login shows when they end.",
		Groups: []workflow.Group{
			{Category: "security", Core: true, Files: []string{"app/session.py"}, Experts: []workflow.Pick{{Name: "correctness"}, {Name: "security"}}},
			{Category: "api", Files: []string{"app/login.py"}, Experts: []workflow.Pick{{Name: "security"}}},
		},
		Confidence: 0.9,
	}, nil
}

func repro(oracle workflow.Oracle, body string) *workflow.Repro {
	return &workflow.Repro{Test: workflow.Edit{Path: "app/test_session.py", New: body}, Run: "app/test_session.py::test_boundary", Oracle: oracle, Quote: "a session expires at expires_at"}
}

func finding(path string, line int, ev workflow.EvidenceKind, r *workflow.Repro) workflow.Claim {
	return workflow.Claim{Skill: "other", Category: "boundary", Path: path, Line: line, Severity: workflow.SeverityHigh, Subject: "valid at expires_at", Discussion: "compares with >", Scenario: "is_expired(Session(expires_at=100), now=100) returns False", Evidence: ev, Repro: r}
}

func answers(t workflow.ReviewTask, findings ...workflow.Claim) workflow.Report {
	return workflow.Report{Findings: findings, Loaded: t.Skills}
}

func base() *agents {
	return &agents{
		diagnose: func() (workflow.Diagnosis, error) {
			return workflow.Diagnosis{Failures: []workflow.Failure{{Job: "test", Cause: "a test fails"}}}, nil
		},
		plan: goodPlan,
		review: func(t workflow.ReviewTask) (workflow.Report, error) {
			if t.Expert.Name == "correctness" {
				return answers(t, finding("app/session.py", 10, workflow.EvidenceReproTest, repro(workflow.OracleSpec, "real"))), nil
			}
			return answers(t), nil
		},
		fix: func(t workflow.FixTask) ([]workflow.Edit, error) {
			if t.Finding.Discussion != "" {
				return nil, errors.New("the fix agent saw the expert's reasoning")
			}
			return []workflow.Edit{{Path: "app/session.py", Old: "bug", New: "ok"}}, nil
		},
		judge: func(workflow.JudgeTask) (workflow.Judgement, error) {
			return workflow.Judgement{Valid: false, Reason: "the code is right"}, nil
		},
		summarize: func(t workflow.SummaryTask) ([]workflow.Draft, error) {
			d := make([]workflow.Draft, 0, len(t.Findings))
			for _, m := range t.Findings {
				d = append(d, workflow.Draft{Finding: m.ID, Label: workflow.LabelIssue, Short: m.Findings[0].Subject, Subject: m.Findings[0].Subject})
			}
			return d, nil
		},
	}
}

func run(t *testing.T, a *agents, ws *workspace, ar *archive, req workflow.Request) workflow.Result {
	t.Helper()
	w, err := workflow.New(slog.New(slog.DiscardHandler), roster)
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Run(t.Context(), workflow.Session{Agents: a, Workspace: ws, Archive: ar}, req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestRun(t *testing.T) {
	t.Run("verified fix", func(t *testing.T) {
		a := base()
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if res.Outcome != workflow.OutcomeComplete || res.PlanSource != workflow.PlanFromPlanner {
			t.Fatalf("outcome %s, plan %s", res.Outcome, res.PlanSource)
		}
		if len(res.Comments) != 1 {
			t.Fatalf("comments %+v", res.Comments)
		}
		c := res.Comments[0]
		if c.Label != workflow.LabelIssue || !slices.Contains(c.Decorations, "blocking") || c.Strength != workflow.StrengthStrong || len(c.Fix) != 1 || c.Test == nil {
			t.Fatalf("comment %+v", c)
		}
		if a.n(workflow.RoleExpert) != 2 || len(res.Runs) != 5 {
			t.Fatalf("expert calls %d, runs %d", a.n(workflow.RoleExpert), len(res.Runs))
		}
	})

	t.Run("agent oracle cannot block", func(t *testing.T) {
		a := base()
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			if t.Expert.Name == "correctness" {
				return answers(t, finding("app/session.py", 10, workflow.EvidenceReproTest, repro(workflow.OracleAgent, "real"))), nil
			}
			return answers(t), nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(res.Comments) != 1 || res.Comments[0].Label != workflow.LabelQuestion || !slices.Contains(res.Comments[0].Decorations, "non-blocking") {
			t.Fatalf("comments %+v", res.Comments)
		}
		if !strings.Contains(strings.Join(res.Uncertainty.Notes, " "), "issue needs strong evidence") {
			t.Fatalf("notes %v", res.Uncertainty.Notes)
		}
	})

	t.Run("not reproduced is dismissed", func(t *testing.T) {
		a := base()
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			if t.Expert.Name == "correctness" {
				return answers(t, finding("app/session.py", 10, workflow.EvidenceReproTest, repro(workflow.OracleSpec, "imagined"))), nil
			}
			return answers(t), nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(res.Comments) != 0 || len(res.Uncertainty.Dismissed) != 1 || a.n(workflow.RoleFix) != 0 {
			t.Fatalf("comments %+v, dismissed %+v", res.Comments, res.Uncertainty.Dismissed)
		}
	})

	t.Run("fix that breaks the suite", func(t *testing.T) {
		res := run(t, base(), &workspace{ci: workflow.Check{Passed: true}, baseline: []string{"flaky"}, broken: []string{"test_login"}}, &archive{}, request())
		v := res.Verifications[0]
		if v.Status != workflow.StatusReproduced || !strings.Contains(v.Reason, "test_login") || res.Comments[0].Fix != nil {
			t.Fatalf("verification %+v", v)
		}
	})

	t.Run("ci failed", func(t *testing.T) {
		a := base()
		res := run(t, a, &workspace{ci: workflow.Check{Passed: false, Log: "FAIL"}}, &archive{}, request())
		if res.Outcome != workflow.OutcomeCIFailed || len(res.Diagnosis.Failures) != 1 || a.n(workflow.RolePlanner) != 0 {
			t.Fatalf("result %+v", res)
		}
	})

	t.Run("empty change", func(t *testing.T) {
		req := request()
		req.Change = workflow.Change{}
		if res := run(t, base(), &workspace{}, &archive{}, req); res.Outcome != workflow.OutcomeNone {
			t.Fatalf("outcome %s", res.Outcome)
		}
	})

	t.Run("plan rejected twice", func(t *testing.T) {
		a := base()
		var rejected []string
		a.plan = func(t workflow.PlanTask) (workflow.Plan, error) {
			rejected = t.Rejected
			return workflow.Plan{Groups: []workflow.Group{{Files: []string{"app/session.py", "uv.lock"}, Experts: []workflow.Pick{{Name: "performance"}}}}, Confidence: 0.9}, nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if res.PlanSource != workflow.PlanFromDefault || a.n(workflow.RolePlanner) != 2 || len(rejected) != 5 {
			t.Fatalf("source %s, calls %d, rejected %v", res.PlanSource, a.n(workflow.RolePlanner), rejected)
		}
		if g := res.Plan.Groups[0]; len(g.Files) != 2 || len(g.Experts) != 2 {
			t.Fatalf("default plan %+v", res.Plan)
		}
	})

	t.Run("low confidence raises the level", func(t *testing.T) {
		a := base()
		a.plan = func(t workflow.PlanTask) (workflow.Plan, error) {
			p, err := goodPlan(t)
			p.Confidence = 0.2
			return p, err
		}
		if res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request()); res.Level != workflow.LevelDeep {
			t.Fatalf("level %s", res.Level)
		}
	})

	t.Run("invalid report drops the expert", func(t *testing.T) {
		a := base()
		var mu sync.Mutex
		var rejected []string
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			switch t.Expert.Name {
			case "security":
				mu.Lock()
				if len(t.Rejected) > 0 {
					rejected = t.Rejected
				}
				mu.Unlock()
				return workflow.Report{Findings: []workflow.Claim{finding("app/elsewhere.py", 1, workflow.EvidenceArgument, nil)}}, nil
			default:
				return workflow.Report{}, errors.New("timeout")
			}
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(res.Uncertainty.Absent) != 2 || len(rejected) != 2 || res.Outcome != workflow.OutcomeComplete {
			t.Fatalf("absent %+v, rejected %v", res.Uncertainty.Absent, rejected)
		}
	})

	t.Run("budget runs out", func(t *testing.T) {
		req := request()
		req.Budget = workflow.Budget{Tokens: 100}
		res := run(t, base(), &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
		if res.Outcome != workflow.OutcomePartial || !res.Uncertainty.BudgetExhausted || res.PlanSource != workflow.PlanFromPlanner {
			t.Fatalf("result %+v", res)
		}
		if len(res.Uncertainty.Absent) != 2 {
			t.Fatalf("absent %+v", res.Uncertainty.Absent)
		}
	})

	t.Run("setup failure leaves findings unverified", func(t *testing.T) {
		a := base()
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}, setup: errors.New("no database")}, &archive{}, request())
		if len(res.Uncertainty.Unverified) != 1 || a.n(workflow.RoleFix) != 0 || res.Comments[0].Label != workflow.LabelQuestion {
			t.Fatalf("result %+v", res.Uncertainty)
		}
	})

	t.Run("read-only judge", func(t *testing.T) {
		a := base()
		req := request()
		req.Verification = workflow.ModeReadOnly
		ws := &workspace{ci: workflow.Check{Passed: true}}
		res := run(t, a, ws, &archive{}, req)
		if len(res.Uncertainty.Dismissed) != 1 || a.n(workflow.RoleJudge) != 1 || ws.tests != 0 {
			t.Fatalf("dismissed %+v, tests %d", res.Uncertainty.Dismissed, ws.tests)
		}
	})

	t.Run("quick without signals spends nothing", func(t *testing.T) {
		a := base()
		req := request()
		req.Level = workflow.LevelQuick
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
		if len(res.Runs) != 0 || len(res.Plan.Groups) != 0 || res.PlanSource != workflow.PlanFromRules {
			t.Fatalf("result %+v", res)
		}
	})

	t.Run("quick routes tool hits", func(t *testing.T) {
		a := base()
		req := request()
		req.Level = workflow.LevelQuick
		ws := &workspace{ci: workflow.Check{Passed: true}, tools: []workflow.ToolResult{
			{Tool: workflow.ToolTypecheck, Hits: []workflow.Hit{{Path: "app/session.py", Line: 10}}},
			{Tool: workflow.ToolOpenAPIDiff, Hits: []workflow.Hit{{Path: "spec/openapi.json", Line: 1}}},
		}}
		res := run(t, a, ws, &archive{}, req)
		if a.n(workflow.RolePlanner) != 0 || a.n(workflow.RoleExpert) != 1 || a.tiers[0] != workflow.TierCheap || len(res.Plan.Skipped) != 1 {
			t.Fatalf("plan %+v, tiers %v", res.Plan, a.tiers)
		}
		if a.n(workflow.RoleFix) != 0 || res.Verifications[0].Status != workflow.StatusUnverified {
			t.Fatalf("verifications %+v", res.Verifications)
		}
	})

	t.Run("requested expert", func(t *testing.T) {
		a := base()
		req := request()
		req.Experts = []string{"security"}
		ws := &workspace{ci: workflow.Check{Passed: true}, tools: []workflow.ToolResult{{Tool: workflow.ToolTypecheck, Hits: []workflow.Hit{{Path: "app/session.py"}}}}}
		res := run(t, a, ws, &archive{}, req)
		if a.n(workflow.RolePlanner) != 0 || res.PlanSource != workflow.PlanFromRequest || a.n(workflow.RoleExpert) != 1 {
			t.Fatalf("source %s, planner %d, experts %d", res.PlanSource, a.n(workflow.RolePlanner), a.n(workflow.RoleExpert))
		}
		if g := res.Plan.Groups[0]; len(g.Files) != 2 || len(g.Experts) != 1 {
			t.Fatalf("plan %+v", res.Plan)
		}
		w, err := workflow.New(slog.New(slog.DiscardHandler), roster)
		if err != nil {
			t.Fatal(err)
		}
		req.Experts = []string{"style"}
		if _, err := w.Run(t.Context(), workflow.Session{}, req); !errors.Is(err, workflow.ErrInvalidRequest) {
			t.Fatalf("unknown expert: %v", err)
		}
	})

	t.Run("skills offered and read", func(t *testing.T) {
		a := base()
		var mu sync.Mutex
		offered := map[string][]string{}
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			mu.Lock()
			offered[t.Expert.Name+":"+strings.Join(t.Group.Files, ",")] = t.Skills
			mu.Unlock()
			if t.Expert.Name == "security" {
				return workflow.Report{Loaded: []string{}}, nil
			}
			return workflow.Report{Loaded: t.Skills, Findings: []workflow.Claim{{Skill: "golang-concurrency", Path: t.Group.Files[0], Line: 1, Severity: workflow.SeverityLow, Subject: "s", Evidence: workflow.EvidenceArgument}}}, nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if got := offered["correctness:app/session.py"]; len(got) != 1 || got[0] != "code-review" {
			t.Fatalf("a Go skill was offered for Python files: %v", got)
		}
		if len(res.Uncertainty.Absent) != 1 || !strings.Contains(res.Uncertainty.Absent[0].Reason, "golang-concurrency") {
			t.Fatalf("a finding citing a skill not offered was accepted: %+v", res.Uncertainty.Absent)
		}
		if len(res.Uncertainty.Unread) != 1 || res.Uncertainty.Unread[0].Skill != "security-review" {
			t.Fatalf("unread %+v", res.Uncertainty.Unread)
		}
	})

	t.Run("a secret starts security", func(t *testing.T) {
		a := base()
		a.plan = func(workflow.PlanTask) (workflow.Plan, error) {
			return workflow.Plan{Groups: []workflow.Group{{Files: []string{"app/session.py", "app/login.py"}, Experts: []workflow.Pick{{Name: "correctness"}}}}, Confidence: 0.9}, nil
		}
		ws := &workspace{ci: workflow.Check{Passed: true}, tools: []workflow.ToolResult{{Tool: workflow.ToolSecretScan, Hits: []workflow.Hit{{Path: "app/login.py", Line: 3, Rule: "aws-key"}}}}}
		res := run(t, a, ws, &archive{}, request())
		if g := res.Plan.Groups[0]; len(g.Experts) != 2 || g.Experts[1].Name != "security" {
			t.Fatalf("plan %+v", res.Plan)
		}
	})
}

func TestBackedEvidence(t *testing.T) {
	for name, tools := range map[string][]workflow.ToolResult{
		"claimed":    nil,
		"other file": {{Tool: workflow.ToolDependencyAudit, Hits: []workflow.Hit{{Path: "uv.lock"}}}},
		"audited":    {{Tool: workflow.ToolDependencyAudit, Hits: []workflow.Hit{{Path: "app/login.py", Rule: "CVE-2026-1"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			a := base()
			a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
				if t.Expert.Name == "security" && slices.Contains(t.Group.Files, "app/login.py") {
					return answers(t, finding("app/login.py", 1, workflow.EvidenceCVE, nil)), nil
				}
				return answers(t), nil
			}
			res := run(t, a, &workspace{ci: workflow.Check{Passed: true}, tools: tools}, &archive{}, request())
			blocking := slices.ContainsFunc(res.Comments, func(c workflow.Comment) bool { return slices.Contains(c.Decorations, "blocking") })
			if blocking != (name == "audited") {
				t.Fatalf("blocking %v, absent %+v", blocking, res.Uncertainty.Absent)
			}
		})
	}
}

func TestScenarioAndShort(t *testing.T) {
	task := workflow.ReviewTask{
		Expert: workflow.Expert{Name: "correctness", Evidence: []workflow.EvidenceKind{workflow.EvidenceReproTest, workflow.EvidenceArgument}},
		Group:  workflow.Group{Files: []string{"a.go"}},
	}
	bug := workflow.Claim{Skill: "other", Category: "c", Path: "a.go", Line: 1, Severity: workflow.SeverityHigh, Subject: "s", Evidence: workflow.EvidenceReproTest, Repro: repro(workflow.OracleImplicit, "real")}
	if reasons := workflow.Validate(task, workflow.Report{Findings: []workflow.Claim{bug}}); len(reasons) != 1 || !strings.Contains(reasons[0], "scenario") {
		t.Fatalf("a bug without a scenario: %v", reasons)
	}
	bug.Evidence, bug.Repro = workflow.EvidenceArgument, nil
	if reasons := workflow.Validate(task, workflow.Report{Findings: []workflow.Claim{bug}}); len(reasons) != 0 {
		t.Fatalf("an argument needs no scenario: %v", reasons)
	}

	a := base()
	long := strings.Repeat("word ", 30)
	a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
		if t.Expert.Name == "correctness" {
			f := finding("app/session.py", 10, workflow.EvidenceArgument, nil)
			f.Subject = long
			return answers(t, f), nil
		}
		return answers(t), nil
	}
	a.summarize = func(workflow.SummaryTask) ([]workflow.Draft, error) { return nil, errors.New("down") }
	res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
	if c := res.Comments[0]; len([]rune(c.Short)) > 60 || !strings.HasSuffix(c.Short, "…") || c.Scenario == "" {
		t.Fatalf("fallback short %q, scenario %q", c.Short, c.Scenario)
	}
}

func TestResume(t *testing.T) {
	ar := &archive{}
	first := base()
	want := run(t, first, &workspace{ci: workflow.Check{Passed: true}}, ar, request())

	again := base()
	got := run(t, again, &workspace{ci: workflow.Check{Passed: true}}, ar, request())
	if n := again.n(workflow.RolePlanner) + again.n(workflow.RoleExpert) + again.n(workflow.RoleFix) + again.n(workflow.RoleSummarizer); n != 0 {
		t.Fatalf("resumed review repeated %d agent calls", n)
	}
	if len(got.Runs) != len(want.Runs) || len(got.Comments) != len(want.Comments) {
		t.Fatalf("runs %d vs %d, comments %d vs %d", len(got.Runs), len(want.Runs), len(got.Comments), len(want.Comments))
	}

	ar = &archive{}
	ctx, cancel := context.WithCancel(t.Context())
	cut := base()
	cut.fix = func(workflow.FixTask) ([]workflow.Edit, error) {
		cancel()
		return nil, context.Canceled
	}
	w, err := workflow.New(slog.New(slog.DiscardHandler), roster)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Run(ctx, workflow.Session{Agents: cut, Workspace: &workspace{ci: workflow.Check{Passed: true}}, Archive: ar}, request()); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v", err)
	}
	resumed := base()
	res := run(t, resumed, &workspace{ci: workflow.Check{Passed: true}}, ar, request())
	if resumed.n(workflow.RoleExpert) != 0 || resumed.n(workflow.RoleFix) != 1 || res.Verifications[0].Status != workflow.StatusFixed {
		t.Fatalf("experts %d, fixes %d, %+v", resumed.n(workflow.RoleExpert), resumed.n(workflow.RoleFix), res.Verifications)
	}
}

func TestNew(t *testing.T) {
	for name, e := range map[string]workflow.Expert{
		"no name":      {Tier: workflow.TierCheap, Evidence: []workflow.EvidenceKind{workflow.EvidenceArgument}},
		"bad tier":     {Name: "x", Tier: "huge", Evidence: []workflow.EvidenceKind{workflow.EvidenceArgument}},
		"other listed": {Name: "x", Tier: workflow.TierCheap, Skills: []workflow.Skill{{Name: "other"}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceArgument}},
		"bad glob":     {Name: "x", Tier: workflow.TierCheap, Skills: []workflow.Skill{{Name: "s", Files: []string{"[x"}}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceArgument}},
		"no evidence":  {Name: "x", Tier: workflow.TierCheap},
	} {
		if _, err := workflow.New(slog.Default(), []workflow.Expert{e}); !errors.Is(err, workflow.ErrInvalidRoster) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func large() workflow.Request {
	req := request()
	req.Change.Files = []workflow.File{
		{Path: "api/a.go", Added: 1500, Diff: "+a\n"},
		{Path: "api/b.go", Added: 900, Diff: "+b\n"},
		{Path: "web/c.ts", Added: 300, Diff: "+c\n"},
		{Path: "web/d.ts", Added: 200, Diff: "+d\n"},
		{Path: "README.md", Added: 50, Diff: "+r\n"},
		{Path: "data/huge.txt", Added: 10, Diff: "+" + strings.Repeat("x", 600<<10) + "\n"},
	}
	return req
}

func scopePlan(t workflow.PlanTask) (workflow.Plan, error) {
	g := workflow.Group{Category: "core", Experts: []workflow.Pick{{Name: "correctness"}}}
	for _, e := range t.Manifest {
		if e.Class == workflow.ClassCode || e.Class == workflow.ClassDocs {
			g.Files = append(g.Files, e.Path)
		}
	}
	return workflow.Plan{Summary: "a change across the repository", Groups: []workflow.Group{g}, Confidence: 0.9}, nil
}

func TestScopes(t *testing.T) {
	t.Run("a large change is split into scopes, each planned and reviewed on its own", func(t *testing.T) {
		a := base()
		a.plan = scopePlan
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) { return answers(t), nil }
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, large())
		if len(res.Scopes) != 2 || a.n(workflow.RolePlanner) != 2 || a.n(workflow.RoleExpert) != 2 {
			t.Fatalf("scopes %+v, planner %d, experts %d", res.Scopes, a.n(workflow.RolePlanner), a.n(workflow.RoleExpert))
		}
		seen := map[string]int{}
		for i, s := range res.Scopes {
			if s.Lines > 2000 {
				t.Errorf("scope %s has %d lines", s.Name, s.Lines)
			}
			for _, f := range s.Files {
				seen[f]++
			}
			if res.Plan.Groups[i].Scope != i || !slices.Equal(slices.Sorted(slices.Values(res.Plan.Groups[i].Files)), s.Files) {
				t.Errorf("group %d %+v does not match scope %+v", i, res.Plan.Groups[i], s)
			}
		}
		if len(seen) != 5 || slices.ContainsFunc(slices.Collect(maps.Values(seen)), func(n int) bool { return n != 1 }) {
			t.Fatalf("files across scopes %v", seen)
		}
		if res.Scopes[0].Name != "/, api (part 1 of 2)" || res.Scopes[1].Name != "api (part 2 of 2), web" {
			t.Fatalf("names %q, %q", res.Scopes[0].Name, res.Scopes[1].Name)
		}
		if !slices.ContainsFunc(res.Triage.Excluded, func(e workflow.Entry) bool {
			return e.Path == "data/huge.txt" && e.Class == workflow.ClassOversized
		}) {
			t.Fatalf("excluded %+v", res.Triage.Excluded)
		}
	})

	t.Run("a scope that spends its share stops, and the next scope still runs", func(t *testing.T) {
		a := base()
		a.plan = func(t workflow.PlanTask) (workflow.Plan, error) {
			if slices.ContainsFunc(t.Manifest, func(e workflow.Entry) bool { return e.Path == "api/a.go" }) {
				return workflow.Plan{Groups: []workflow.Group{}, Confidence: 0.9}, nil
			}
			return scopePlan(t)
		}
		var reviewed [][]string
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			reviewed = append(reviewed, t.Group.Files)
			return answers(t), nil
		}
		req := large()
		req.Budget = workflow.Budget{Tokens: 400}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
		if res.Outcome != workflow.OutcomePartial || len(reviewed) != 1 || !slices.Contains(reviewed[0], "web/c.ts") {
			t.Fatalf("outcome %s, reviewed %v, notes %v", res.Outcome, reviewed, res.Uncertainty.Notes)
		}
		if !slices.ContainsFunc(res.Uncertainty.Notes, func(n string) bool { return strings.Contains(n, "used up its share") }) {
			t.Fatalf("notes %v", res.Uncertainty.Notes)
		}
	})

	t.Run("a small change is one scope", func(t *testing.T) {
		res := run(t, base(), &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(res.Scopes) != 1 || res.Scopes[0].Name != "app" {
			t.Fatalf("scopes %+v", res.Scopes)
		}
	})
}

func TestPlannerDiffLimit(t *testing.T) {
	a := base()
	var seen workflow.PlanTask
	a.plan = func(t workflow.PlanTask) (workflow.Plan, error) {
		seen = t
		return goodPlan(t)
	}
	var mu sync.Mutex
	var testOne string
	a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
		mu.Lock()
		defer mu.Unlock()
		testOne = t.TestOne
		return answers(t), nil
	}
	req := request()
	req.Project.TestOne = "pytest -k {test}"
	req.Change.Files[1].Diff = "+" + strings.Repeat("x", 200<<10) + "\n"
	run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
	if !seen.Truncated || len(seen.Files) != 1 || seen.Files[0].Path != "app/session.py" {
		t.Fatalf("planner saw %d files, truncated %v", len(seen.Files), seen.Truncated)
	}
	if testOne != "pytest -k {test}" {
		t.Fatalf("expert got test_one %q", testOne)
	}
}

func TestParallel(t *testing.T) {
	files := make([]workflow.File, 6)
	groups := make([]workflow.Group, 6)
	panel := make([]workflow.Expert, 6)
	for i := range files {
		files[i] = workflow.File{Path: fmt.Sprintf("app/m%d.py", i), Added: 1, Diff: "+x\n"}
		panel[i] = workflow.Expert{Name: fmt.Sprintf("e%d", i), Tier: workflow.TierMid, Evidence: []workflow.EvidenceKind{workflow.EvidenceArgument}}
	}
	setup := func(experts []string, budget int64) (*agents, *atomic.Int32, *workflow.Workflow, workflow.Request) {
		for i := range groups {
			picks := make([]workflow.Pick, 0, len(experts))
			for _, e := range experts {
				picks = append(picks, workflow.Pick{Name: e})
			}
			groups[i] = workflow.Group{Category: "core", Files: []string{files[i].Path}, Experts: picks}
		}
		a := base()
		a.plan = func(workflow.PlanTask) (workflow.Plan, error) {
			return workflow.Plan{Summary: "six modules", Groups: groups, Confidence: 0.9}, nil
		}
		var running, peak atomic.Int32
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			n := running.Add(1)
			for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
			}
			time.Sleep(50 * time.Millisecond)
			running.Add(-1)
			return answers(t), nil
		}
		w, err := workflow.New(slog.New(slog.DiscardHandler), slices.Concat(roster, panel))
		if err != nil {
			t.Fatal(err)
		}
		req := request()
		req.Change = workflow.Change{Files: files, Commits: []string{}}
		req.Budget = workflow.Budget{Tokens: budget}
		return a, &peak, w, req
	}
	review := func(w *workflow.Workflow, a *agents, req workflow.Request) workflow.Result {
		res, err := w.Run(t.Context(), workflow.Session{Agents: a, Workspace: &workspace{ci: workflow.Check{Passed: true}}, Archive: &archive{}}, req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	t.Run("each expert runs once, however many groups it reviews", func(t *testing.T) {
		a, _, w, req := setup([]string{"correctness", "security"}, 1_000_000)
		var mu sync.Mutex
		seen := map[string][]string{}
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			mu.Lock()
			seen[t.Expert.Name] = t.Group.Files
			mu.Unlock()
			return answers(t), nil
		}
		review(w, a, req)
		if a.n(workflow.RoleExpert) != 2 || len(seen["correctness"]) != 6 || len(seen["security"]) != 6 {
			t.Fatalf("experts %d, files %v", a.n(workflow.RoleExpert), seen)
		}
	})

	t.Run("at most four experts run at once", func(t *testing.T) {
		a, peak, w, req := setup([]string{"e0", "e1", "e2", "e3", "e4", "e5"}, 1_000_000)
		review(w, a, req)
		if a.n(workflow.RoleExpert) != 6 || peak.Load() != 4 {
			t.Fatalf("experts %d, peak %d", a.n(workflow.RoleExpert), peak.Load())
		}
	})

	t.Run("judges run at once", func(t *testing.T) {
		a, _, w, req := setup([]string{"correctness"}, 1_000_000)
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			claims := make([]workflow.Claim, 0, len(t.Group.Files))
			for _, f := range t.Group.Files {
				claims = append(claims, finding(f, 1, workflow.EvidenceArgument, nil))
			}
			return answers(t, claims...), nil
		}
		var running, peak atomic.Int32
		a.judge = func(workflow.JudgeTask) (workflow.Judgement, error) {
			n := running.Add(1)
			for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
			}
			time.Sleep(50 * time.Millisecond)
			running.Add(-1)
			return workflow.Judgement{Valid: true, Reason: "r"}, nil
		}
		req.Verification = workflow.ModeReadOnly
		res := review(w, a, req)
		if a.n(workflow.RoleJudge) != 6 || peak.Load() != 4 || len(res.Verifications) != 6 {
			t.Fatalf("judges %d, peak %d, verifications %d", a.n(workflow.RoleJudge), peak.Load(), len(res.Verifications))
		}
	})

	t.Run("a spent budget stops the experts still waiting", func(t *testing.T) {
		a, _, w, req := setup([]string{"e0", "e1", "e2", "e3", "e4", "e5"}, 150)
		res := review(w, a, req)
		if res.Outcome != workflow.OutcomePartial || a.n(workflow.RoleExpert) != 4 {
			t.Fatalf("outcome %s, experts %d", res.Outcome, a.n(workflow.RoleExpert))
		}
	})
}

func TestThoroughness(t *testing.T) {
	for _, tt := range []struct {
		level workflow.Level
		kept  int
	}{
		{workflow.LevelStandard, 0},
		{workflow.LevelDeep, 1},
	} {
		t.Run(string(tt.level), func(t *testing.T) {
			a := base()
			var levels []workflow.Level
			var mu sync.Mutex
			a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
				mu.Lock()
				levels = append(levels, t.Level)
				mu.Unlock()
				if t.Expert.Name != "correctness" {
					return answers(t), nil
				}
				return answers(t, workflow.Claim{Skill: "other", Category: "naming", Path: "app/session.py", Line: 2, Severity: workflow.SeverityLow, Subject: "rename x", Evidence: workflow.EvidenceArgument}), nil
			}
			req := request()
			req.Level, req.Verification = tt.level, workflow.ModeNone
			res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
			if len(res.Findings) != tt.kept || slices.ContainsFunc(levels, func(l workflow.Level) bool { return l != tt.level }) {
				t.Fatalf("findings %+v, levels %v", res.Findings, levels)
			}
		})
	}

	t.Run("a risky path makes its group core", func(t *testing.T) {
		a := base()
		a.plan = func(workflow.PlanTask) (workflow.Plan, error) {
			p, err := goodPlan(workflow.PlanTask{})
			p.Groups[0].Core = false
			return p, err
		}
		req := request()
		req.Level, req.Project.Risk = workflow.LevelStandard, []string{"app/session.py"}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
		if !res.Plan.Groups[0].Core || res.Plan.Groups[1].Core {
			t.Fatalf("groups %+v", res.Plan.Groups)
		}
	})
}
