package workflow_test

import (
	"cmp"
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
	"github.com/Pleasurecruise/eyeful/workflow/pulls"
)

type agents struct {
	mu       sync.Mutex
	calls    map[workflow.Role]int
	tiers    []workflow.Tier
	diagnose func() (workflow.Diagnosis, error)
	plan     func(t workflow.PlanTask) (pulls.Analysis, error)
	review   func(t workflow.ReviewTask) (workflow.Report, error)
	fix      func(t workflow.FixTask) ([]workflow.Edit, error)
	retest   func(t workflow.RetestTask) (workflow.Repro, error)
	judge    func(t workflow.JudgeTask) ([]workflow.Judgement, error)
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

func (a *agents) Plan(_ context.Context, t workflow.PlanTask) (pulls.Analysis, workflow.Usage, error) {
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

func (a *agents) Retest(_ context.Context, t workflow.RetestTask) (workflow.Repro, workflow.Usage, error) {
	a.count(workflow.RoleRepro)
	r, err := a.retest(t)
	return r, usage, err
}

func (a *agents) Judge(_ context.Context, t workflow.JudgeTask) ([]workflow.Judgement, workflow.Usage, error) {
	a.count(workflow.RoleJudge)
	j, err := a.judge(t)
	return j, usage, err
}

type workspace struct {
	ci        workflow.Check
	setup     error
	broken    []string
	baseline  []string
	regressed []string
	tests     int
	flaky     bool
	old       bool
	added     bool
	output    string
}

func (w *workspace) CI(context.Context) (workflow.Check, error) { return w.ci, nil }
func (w *workspace) Setup(context.Context) error                { return w.setup }

func (w *workspace) Read(_ context.Context, path string) (string, error) {
	if path != "docs/sessions.md" {
		return "", errors.New("no such file")
	}
	return "A session expires\n    at expires_at.", nil
}

func (w *workspace) Test(_ context.Context, edits []workflow.Edit, t workflow.Target) (workflow.TestRun, error) {
	w.tests++
	fixed := slices.ContainsFunc(edits, func(e workflow.Edit) bool { return e.Old == "bug" })
	if len(t.Tests) == 0 {
		failed := w.baseline
		switch {
		case fixed:
			failed = slices.Concat(w.baseline, w.broken)
		case !t.Base:
			failed = slices.Concat(w.baseline, w.regressed)
		}
		return workflow.TestRun{Passed: len(failed) == 0, Failed: failed}, nil
	}
	reproduces := !slices.ContainsFunc(edits, func(e workflow.Edit) bool { return e.Old == "" }) || slices.ContainsFunc(edits, func(e workflow.Edit) bool { return strings.Contains(e.New, "real") })
	run := workflow.TestRun{Passed: fixed || !reproduces, Output: cmp.Or(w.output, "AssertionError: still valid at expires_at")}
	switch {
	case t.Base && w.added:
		run.Passed, run.Output = false, "ImportError: cannot import name is_expired"
	case t.Base:
		run.Passed = !w.old
	case w.flaky && w.tests%2 == 0:
		run.Passed = true
	}
	return run, nil
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
	{Name: "correctness", Area: "correctness", Tier: workflow.TierStrong, Skills: []workflow.Skill{{Name: "code-review"}, {Name: "golang-concurrency", Files: []string{"**/*.go"}}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceTest, workflow.EvidenceInspection}},
	{Name: "security", Area: "security", Tier: workflow.TierStrong, Skills: []workflow.Skill{{Name: "security-review"}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceTest, workflow.EvidenceAnalysis}},
}

func gitDiff(path, line string) string {
	return fmt.Sprintf("diff --git a/%[1]s b/%[1]s\nindex 1111111..2222222 100644\n--- a/%[1]s\n+++ b/%[1]s\n@@ -1 +1 @@\n-old\n+%[2]s\n", path, line)
}

func group(key string, category pulls.DiffCategory, critical bool, paths ...string) pulls.DiffGroup {
	return pulls.DiffGroup{Key: key, Label: key, Category: category, Critical: critical, FilePaths: paths}
}

func change() workflow.Change {
	return workflow.Change{
		Files: []workflow.File{
			{Path: "app/session.py", Added: 3, Deleted: 1, Diff: gitDiff("app/session.py", "bug")},
			{Path: "app/login.py", Added: 5, Deleted: 0, Diff: gitDiff("app/login.py", "x")},
			{Path: "uv.lock", Added: 40, Deleted: 2},
		},
		Commits: []string{"fix session expiry"},
	}
}

func request() workflow.Request {
	return workflow.Request{
		Change: change(), Verification: workflow.ModeExecution,
		Budget:  workflow.Budget{Tokens: 1_000_000, MicroUSD: 1_000_000},
		Project: workflow.Project{TestOne: "pytest {test}"},
	}
}

func goodAnalysis(workflow.PlanTask) (pulls.Analysis, error) {
	return pulls.Analysis{
		OverallSummary: "Sessions expire at the exact expiry second, and login shows when they end.",
		Groups:         []pulls.DiffGroup{group("expiry", "security", true, "app/session.py"), group("login", "api", false, "app/login.py")},
	}, nil
}

func repro(oracle workflow.Oracle, body string) *workflow.Repro {
	return &workflow.Repro{Test: workflow.Edit{Path: "app/test_session.py", New: body}, Run: "app/test_session.py::test_boundary", Fails: "still valid at expires_at", Oracle: oracle, Quote: "session expires at expires_at", Source: "docs/sessions.md"}
}

func writes(first *workflow.Repro) func(workflow.RetestTask) (workflow.Repro, error) {
	return func(t workflow.RetestTask) (workflow.Repro, error) {
		if t.Repro != nil {
			return workflow.Repro{}, errors.New("no better reproduction")
		}
		return *first, nil
	}
}

func finding(path string, line int, ev workflow.EvidenceKind) workflow.Claim {
	return workflow.Claim{Skill: "other", Category: "boundary", Path: path, Line: line, Severity: workflow.SeverityError, Subject: "valid at expires_at", Discussion: "compares with >", Scenario: "is_expired(Session(expires_at=100), now=100) returns False", Evidence: ev}
}

func verdicts(t workflow.JudgeTask, valid bool, reason string) []workflow.Judgement {
	out := make([]workflow.Judgement, 0, len(t.Findings))
	for _, f := range t.Findings {
		out = append(out, workflow.Judgement{Finding: f.ID, Valid: valid, Reason: reason})
	}
	return out
}

func answers(t workflow.ReviewTask, findings ...workflow.Claim) workflow.Report {
	return workflow.Report{Findings: findings, Loaded: t.Skills}
}

func base() *agents {
	return &agents{
		diagnose: func() (workflow.Diagnosis, error) {
			return workflow.Diagnosis{Failures: []workflow.Failure{{Job: "test", Cause: "a test fails"}}}, nil
		},
		plan: goodAnalysis,
		review: func(t workflow.ReviewTask) (workflow.Report, error) {
			if t.Expert.Name == "correctness" {
				return answers(t, finding("app/session.py", 10, workflow.EvidenceTest)), nil
			}
			return answers(t), nil
		},
		fix: func(t workflow.FixTask) ([]workflow.Edit, error) {
			if t.Finding.Discussion != "" {
				return nil, errors.New("the fix agent saw the expert's reasoning")
			}
			return []workflow.Edit{{Path: "app/session.py", Old: "bug", New: "ok"}}, nil
		},
		retest: writes(repro(workflow.OracleSpecified, "real")),
		judge: func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			return verdicts(t, true, "the change introduces it"), nil
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
		if c.Label != workflow.LabelIssue || !slices.Contains(c.Decorations, "blocking") || c.Kind != workflow.KindFail || len(c.Fix) != 1 || c.Repro == nil {
			t.Fatalf("comment %+v", c)
		}
		if a.n(workflow.RoleExpert) != 2 || a.n(workflow.RoleRepro) != 1 || len(res.Runs) != 5 {
			t.Fatalf("expert calls %d, runs %d", a.n(workflow.RoleExpert), len(res.Runs))
		}
	})

	t.Run("agent oracle cannot block", func(t *testing.T) {
		a := base()
		a.retest = writes(repro(workflow.OracleHuman, "real"))
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(res.Comments) != 1 || res.Comments[0].Label != workflow.LabelQuestion || !slices.Contains(res.Comments[0].Decorations, "non-blocking") {
			t.Fatalf("comments %+v", res.Comments)
		}
	})

	t.Run("a finding its test does not show is read by the judge", func(t *testing.T) {
		for valid, objection := range map[bool]string{true: "", false: "read"} {
			kind := map[bool]workflow.Kind{true: workflow.KindReview, false: workflow.KindOpen}[valid]
			a := base()
			a.retest = writes(repro(workflow.OracleSpecified, "imagined"))
			var tried workflow.Verification
			a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
				tried = t.Tried[0]
				return verdicts(t, valid, "read"), nil
			}
			res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
			v := res.Verifications[0]
			if len(res.Comments) != 1 || res.Comments[0].Objection != objection || res.Comments[0].Kind != kind || a.n(workflow.RoleFix) != 0 || tried.Status != workflow.StatusNotReproduced || tried.Repro == nil {
				t.Fatalf("valid %v: comments %+v, the judge was told %+v", valid, res.Comments, tried)
			}
			if v.Repro == nil || !slices.Equal(v.Change, slices.Repeat([]workflow.TestStatus{workflow.TestPassed}, 5)) || v.Reason != "read" {
				t.Fatalf("verification %+v", v)
			}
		}
	})

	t.Run("what the runs show decides the status", func(t *testing.T) {
		for name, tc := range map[string]struct {
			ws     workspace
			status workflow.Status
			change []workflow.TestStatus
			base   workflow.TestStatus
			tests  int
		}{
			"flaky":             {workspace{flaky: true}, workflow.StatusConfirmed, []workflow.TestStatus{workflow.TestPassed, workflow.TestFailed}, "", 3},
			"fails otherwise":   {workspace{output: "SyntaxError: invalid syntax"}, workflow.StatusConfirmed, slices.Repeat([]workflow.TestStatus{workflow.TestError}, 5), "", 6},
			"fails on the base": {workspace{old: true}, workflow.StatusPreExisting, slices.Repeat([]workflow.TestStatus{workflow.TestFailed}, 5), workflow.TestFailed, 7},
		} {
			t.Run(name, func(t *testing.T) {
				a, ws := base(), tc.ws
				ws.ci = workflow.Check{Passed: true}
				res := run(t, a, &ws, &archive{}, request())
				v := res.Verifications[0]
				if v.Status != tc.status || !slices.Equal(v.Change, tc.change) || v.Base != tc.base || ws.tests != tc.tests || a.n(workflow.RoleFix) != 0 {
					t.Fatalf("verification %+v after %d test runs and %d fixes", v, ws.tests, a.n(workflow.RoleFix))
				}
				if len(res.Comments) != 1 || res.Comments[0].Label != workflow.LabelQuestion || slices.Contains(res.Comments[0].Decorations, "blocking") || res.Comments[0].Kind == workflow.KindFail {
					t.Fatalf("comments %+v", res.Comments)
				}
			})
		}
	})

	t.Run("a reproduction that fails otherwise is corrected once", func(t *testing.T) {
		a := base()
		wrong := repro(workflow.OracleSpecified, "real")
		wrong.Fails = "a text the run never prints"
		var shown workflow.RetestTask
		a.retest = func(t workflow.RetestTask) (workflow.Repro, error) {
			if t.Repro == nil {
				return *wrong, nil
			}
			shown = t
			return *repro(workflow.OracleImplicit, "real"), nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		v := res.Verifications[0]
		if a.n(workflow.RoleRepro) != 2 || v.Status != workflow.StatusFixed || v.Repro.Fails != "still valid at expires_at" || v.Repro.Oracle != workflow.OracleImplicit || shown.Repro.Fails != wrong.Fails {
			t.Fatalf("corrections %d, verification %+v", a.n(workflow.RoleRepro), v)
		}
		if shown.Command != "pytest {test}" || !strings.Contains(shown.Output, "AssertionError") || !slices.Contains(res.Comments[0].Decorations, "blocking") {
			t.Fatalf("task %+v, comment %+v", shown, res.Comments[0])
		}
	})

	t.Run("oracles are graded", func(t *testing.T) {
		unquoted := repro(workflow.OracleSpecified, "real")
		unquoted.Source = "docs/other.md"
		crash := repro(workflow.OracleImplicit, "real")
		asserted := repro(workflow.OracleImplicit, "real: still valid at expires_at")
		for name, tc := range map[string]struct {
			repro    *workflow.Repro
			added    bool
			oracle   workflow.Oracle
			blocking bool
		}{
			"a quote found in its source":               {repro(workflow.OracleSpecified, "real"), true, workflow.OracleSpecified, true},
			"a quote missing from its source":           {unquoted, true, workflow.OracleHuman, false},
			"a failure the runtime prints":              {crash, true, workflow.OracleImplicit, true},
			"a failure the test wrote itself":           {asserted, true, workflow.OracleHuman, false},
			"a test already in the repository":          {&workflow.Repro{Run: "app/test_login.py::test_expiry", Fails: "still valid at expires_at", Oracle: workflow.OracleDerived}, false, workflow.OracleDerived, true},
			"a test the base commit passes":             {repro(workflow.OracleDerived, "real"), false, workflow.OracleDerived, false},
			"a test the base commit cannot run":         {repro(workflow.OracleDerived, "real"), true, workflow.OracleHuman, false},
			"the expert's reading, and the base passes": {repro(workflow.OracleHuman, "real"), false, workflow.OracleDerived, false},
		} {
			t.Run(name, func(t *testing.T) {
				a := base()
				a.retest = writes(tc.repro)
				res := run(t, a, &workspace{ci: workflow.Check{Passed: true}, added: tc.added}, &archive{}, request())
				if v := res.Verifications[0]; v.Status != workflow.StatusFixed || v.Repro.Oracle != tc.oracle {
					t.Fatalf("verification %+v, absent %+v", v, res.Uncertainty.Absent)
				}
				if c := res.Comments[0]; slices.Contains(c.Decorations, "blocking") != tc.blocking || c.Repro.Oracle != tc.oracle {
					t.Fatalf("comment %+v", c)
				}
			})
		}
	})

	t.Run("the last review's reproductions run again before anyone reads", func(t *testing.T) {
		last := func(test string) workflow.Result {
			return workflow.Result{
				Findings: []workflow.Finding{
					{ID: "0/correctness/0", Expert: "correctness", Claim: finding("app/session.py", 10, workflow.EvidenceTest)},
					{ID: "0/correctness/1", Expert: "correctness", Claim: finding("app/session.py", 30, workflow.EvidenceInspection)},
				},
				Verifications: []workflow.Verification{
					{Finding: "0/correctness/0", Status: workflow.StatusFixed, Repro: repro(workflow.OracleSpecified, test)},
					{Finding: "0/correctness/1", Status: workflow.StatusConfirmed},
				},
			}
		}
		for name, tc := range map[string]struct {
			test       string
			ws         workspace
			status     workflow.Status
			told       int
			comments   int
			unverified int
		}{
			"red to green is resolved":                 {"imagined", workspace{}, workflow.StatusResolved, 0, 0, 0},
			"still red is carried, not found anew":     {"real", workspace{}, workflow.StatusReproduced, 1, 1, 0},
			"a flaky one is shown as unverified":       {"real", workspace{flaky: true}, workflow.StatusUnverified, 0, 1, 1},
			"red on the new base is not this change's": {"real", workspace{old: true}, workflow.StatusPreExisting, 0, 1, 0},
		} {
			a, req := base(), request()
			req.Previous = last(tc.test)
			var mu sync.Mutex
			told := 0
			a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
				mu.Lock()
				told = max(told, len(t.Earlier))
				mu.Unlock()
				return answers(t), nil
			}
			tc.ws.ci = workflow.Check{Passed: true}
			res := run(t, a, &tc.ws, &archive{}, req)
			if len(res.Verifications) != 1 || res.Verifications[0].Status != tc.status || res.Verifications[0].Finding != "previous/0/correctness/0" || len(res.Findings) != 1 {
				t.Fatalf("%s: verifications %+v, findings %+v", name, res.Verifications, res.Findings)
			}
			if told != tc.told || len(res.Comments) != tc.comments || len(res.Uncertainty.Unverified) != tc.unverified || a.n(workflow.RoleRepro)+a.n(workflow.RoleFix)+a.n(workflow.RoleJudge) != 0 {
				t.Fatalf("%s: experts told %d, comments %+v, unverified %+v, agents %v", name, told, res.Comments, res.Uncertainty.Unverified, a.calls)
			}
			if tc.status == workflow.StatusReproduced && (res.Comments[0].Kind != workflow.KindFail || res.Comments[0].Repro == nil) {
				t.Fatalf("%s: carried comment %+v", name, res.Comments[0])
			}
		}
		req := request()
		req.Verification, req.Previous = workflow.ModeReadOnly, last("real")
		ws, a := &workspace{ci: workflow.Check{Passed: true}}, base()
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) { return answers(t), nil }
		if res := run(t, a, ws, &archive{}, req); len(res.Verifications) != 0 || ws.tests != 0 {
			t.Fatalf("a review that runs nothing rechecked: %+v after %d runs", res.Verifications, ws.tests)
		}
		req = request()
		req.Previous = last("real")
		req.Previous.Findings[0].Severity, req.Previous.Verifications[0].Repro.Oracle = "high", "spec"
		ws = &workspace{ci: workflow.Check{Passed: true}}
		if res := run(t, a, ws, &archive{}, req); len(res.Verifications) != 0 || len(res.Comments) != 0 {
			t.Fatalf("a last review graded on other scales was carried: %+v, %+v", res.Verifications, res.Comments)
		}
	})

	t.Run("a blind expert reviews without the plan", func(t *testing.T) {
		a := base()
		var mu sync.Mutex
		plans := map[string]string{}
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			mu.Lock()
			plans[t.Expert.Name] = t.Plan.Summary + t.Group.Summary
			mu.Unlock()
			return answers(t), nil
		}
		w, err := workflow.New(slog.New(slog.DiscardHandler), append(slices.Clone(roster), workflow.Expert{Name: "skeptic", Tier: workflow.TierStrong, Blind: true, Evidence: []workflow.EvidenceKind{workflow.EvidenceTest}}))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Run(t.Context(), workflow.Session{Agents: a, Workspace: &workspace{ci: workflow.Check{Passed: true}}, Archive: &archive{}}, request()); err != nil {
			t.Fatal(err)
		}
		if plans["skeptic"] != "" || plans["correctness"] == "" || len(plans) != 3 {
			t.Fatalf("what each expert was told of the plan: %q", plans)
		}
	})

	t.Run("an archive resumes its own request only", func(t *testing.T) {
		a, kept := base(), &archive{}
		run(t, a, &workspace{ci: workflow.Check{Passed: true}}, kept, request())
		calls := a.n(workflow.RoleExpert)
		more := request()
		more.Budget.Tokens, more.Verification = 2_000_000, workflow.ModeNone
		run(t, a, &workspace{ci: workflow.Check{Passed: true}}, kept, more)
		if a.n(workflow.RoleExpert) != calls {
			t.Fatalf("a resumed review called its experts again: %d, then %d", calls, a.n(workflow.RoleExpert))
		}
		other := request()
		other.Experts = []string{"security"}
		w, err := workflow.New(slog.New(slog.DiscardHandler), roster)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Run(t.Context(), workflow.Session{Agents: a, Workspace: &workspace{ci: workflow.Check{Passed: true}}, Archive: kept}, other); !errors.Is(err, workflow.ErrOtherRequest) {
			t.Fatalf("another request reused the archive: %v", err)
		}
	})

	t.Run("findings on the same lines are checked together", func(t *testing.T) {
		for name, tc := range map[string]struct {
			test          string
			rejected      []string
			fixes, judges int
			statuses      []workflow.Status
			objection     string
		}{
			"one reproduction stands for both":            {"real", nil, 1, 0, []workflow.Status{workflow.StatusFixed}, ""},
			"one judge reads both":                        {"imagined", nil, 0, 1, []workflow.Status{workflow.StatusConfirmed, workflow.StatusConfirmed}, ""},
			"a rejected one does not take the other down": {"imagined", []string{"correctness"}, 0, 1, []workflow.Status{workflow.StatusRejected, workflow.StatusConfirmed}, ""},
			"both rejected":                               {"imagined", []string{"correctness", "security"}, 0, 1, []workflow.Status{workflow.StatusRejected, workflow.StatusRejected}, "read"},
		} {
			a := base()
			a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
				line := map[string]int{"correctness": 10, "security": 11}[t.Expert.Name]
				return answers(t, finding("app/session.py", line, workflow.EvidenceTest)), nil
			}
			a.retest = writes(repro(workflow.OracleSpecified, tc.test))
			a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
				out := verdicts(t, true, "read")
				for i, f := range t.Findings {
					out[i].Valid = !slices.Contains(tc.rejected, f.Expert)
				}
				return out, nil
			}
			res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
			if a.n(workflow.RoleRepro) != 1 || a.n(workflow.RoleFix) != tc.fixes || a.n(workflow.RoleJudge) != tc.judges {
				t.Fatalf("%s: %d reproductions, %d fixes and %d judgements", name, a.n(workflow.RoleRepro), a.n(workflow.RoleFix), a.n(workflow.RoleJudge))
			}
			statuses := make([]workflow.Status, 0, len(res.Verifications))
			for _, v := range res.Verifications {
				statuses = append(statuses, v.Status)
			}
			if !slices.Equal(statuses, tc.statuses) || len(res.Comments) != 1 || len(res.Comments[0].Experts) != 2 || res.Comments[0].Objection != tc.objection {
				t.Fatalf("%s: statuses %v, comments %+v", name, statuses, res.Comments)
			}
		}
	})

	t.Run("fix that rewrites the test", func(t *testing.T) {
		a := base()
		a.fix = func(workflow.FixTask) ([]workflow.Edit, error) {
			return []workflow.Edit{{Path: "app/session.py", Old: "bug", New: "ok"}, {Path: "./app/test_session.py", Old: "real", New: "pass"}}, nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if v := res.Verifications[0]; v.Status != workflow.StatusReproduced || v.Fixed != "" || res.Comments[0].Fix != nil {
			t.Fatalf("verification %+v, comment %+v", v, res.Comments[0])
		}
	})

	t.Run("conditions share the experts' findings", func(t *testing.T) {
		ar := &archive{}
		run(t, base(), &workspace{ci: workflow.Check{Passed: true}}, ar, request())
		a, req := base(), request()
		a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			return verdicts(t, false, "the code is right"), nil
		}
		req.Verification = workflow.ModeReadOnly
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, ar, req)
		if a.n(workflow.RolePlanner)+a.n(workflow.RoleExpert) != 0 || a.n(workflow.RoleJudge) != 1 || res.Verifications[0].Status != workflow.StatusRejected || len(res.Comments) != 1 || res.Comments[0].Objection != "the code is right" {
			t.Fatalf("planner %d, experts %d, judge %d, %+v", a.n(workflow.RolePlanner), a.n(workflow.RoleExpert), a.n(workflow.RoleJudge), res.Verifications)
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
		a.plan = func(t workflow.PlanTask) (pulls.Analysis, error) {
			rejected = t.Rejected
			return pulls.Analysis{OverallSummary: "s", Groups: []pulls.DiffGroup{group("g", "prose", false, "app/session.py")}}, nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if res.PlanSource != workflow.PlanFromDefault || a.n(workflow.RolePlanner) != 2 || len(rejected) != 1 || !strings.Contains(rejected[0], "category") {
			t.Fatalf("source %s, calls %d, rejected %v", res.PlanSource, a.n(workflow.RolePlanner), rejected)
		}
		if g := res.Plan.Groups[0]; len(g.Files) != 2 || len(g.Experts) != 2 {
			t.Fatalf("default plan %+v", res.Plan)
		}
	})

	t.Run("invalid finding is dropped alone", func(t *testing.T) {
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
				return workflow.Report{Findings: []workflow.Claim{
					finding("app/elsewhere.py", 1, workflow.EvidenceAnalysis),
					finding("app/session.py", 4, workflow.EvidenceAnalysis),
				}}, nil
			default:
				return workflow.Report{}, errors.New("timeout")
			}
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(res.Uncertainty.Absent) != 1 || len(rejected) != 1 || res.Outcome != workflow.OutcomePartial {
			t.Fatalf("absent %+v, rejected %v", res.Uncertainty.Absent, rejected)
		}
		if len(res.Findings) != 1 || res.Findings[0].Line != 4 || !strings.Contains(strings.Join(res.Uncertainty.Notes, " "), "app/elsewhere.py") {
			t.Fatalf("findings %+v, notes %v", res.Findings, res.Uncertainty.Notes)
		}
		if len(res.Uncertainty.Unread) != 0 {
			t.Fatalf("skills counted as unread though nothing observed them: %+v", res.Uncertainty.Unread)
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

	t.Run("after a setup failure the judge reads every finding", func(t *testing.T) {
		a, ws := base(), &workspace{ci: workflow.Check{Passed: true}, setup: errors.New("no database")}
		res := run(t, a, ws, &archive{}, request())
		if len(res.Uncertainty.Skipped) != 1 || ws.tests != 0 || a.n(workflow.RoleRepro)+a.n(workflow.RoleFix) != 0 || a.n(workflow.RoleJudge) != 1 || res.Comments[0].Label != workflow.LabelQuestion {
			t.Fatalf("result %+v after %d test runs", res.Uncertainty, ws.tests)
		}
	})

	t.Run("execution has the judge read what no test proves", func(t *testing.T) {
		a := base()
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			if t.Expert.Name != "correctness" {
				return answers(t), nil
			}
			proved := finding("app/session.py", 10, workflow.EvidenceTest)
			proved.Related = []workflow.Location{{Path: "app/session.py", Line: 40}}
			argued := finding("app/session.py", 30, workflow.EvidenceInspection)
			return answers(t, proved, argued), nil
		}
		var read []int
		a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			for _, f := range t.Findings {
				read = append(read, f.Line)
			}
			return verdicts(t, false, "the base commit does the same"), nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if !slices.Equal(read, []int{30}) || len(res.Comments) != 2 {
			t.Fatalf("the judge read lines %v, comments %+v", read, res.Comments)
		}
		proved := res.Comments[slices.IndexFunc(res.Comments, func(c workflow.Comment) bool { return c.Line == 10 })]
		disputed := res.Comments[slices.IndexFunc(res.Comments, func(c workflow.Comment) bool { return c.Line == 30 })]
		if proved.Objection != "" || !slices.Equal(proved.Related, []workflow.Location{{Path: "app/session.py", Line: 40}}) {
			t.Fatalf("proved %+v", proved)
		}
		if disputed.Objection != "the base commit does the same" || disputed.Kind != workflow.KindOpen || !slices.Contains(disputed.Decorations, "non-blocking") {
			t.Fatalf("disputed %+v", disputed)
		}
	})

	t.Run("a related place is another line of the group's files", func(t *testing.T) {
		a := base()
		var mu sync.Mutex
		var rejected []string
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			mu.Lock()
			rejected = append(rejected, t.Rejected...)
			mu.Unlock()
			c := finding("app/session.py", 10, workflow.EvidenceTest)
			if t.Expert.Name == "correctness" && len(t.Rejected) == 0 {
				c.Related = []workflow.Location{{Path: "app/other.py", Line: 3}, {Path: "app/session.py", Line: 10}}
			}
			return answers(t, c), nil
		}
		run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(rejected) != 2 || !strings.Contains(rejected[0], "app/other.py:3") {
			t.Fatalf("rejected %v", rejected)
		}
	})

	t.Run("read-only judge", func(t *testing.T) {
		a := base()
		a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			return verdicts(t, false, "the code is right"), nil
		}
		req := request()
		req.Verification = workflow.ModeReadOnly
		ws := &workspace{ci: workflow.Check{Passed: true}}
		res := run(t, a, ws, &archive{}, req)
		if len(res.Comments) != 1 || res.Comments[0].Objection != "the code is right" || a.n(workflow.RoleJudge) != 1 || ws.tests != 0 {
			t.Fatalf("comments %+v, tests %d", res.Comments, ws.tests)
		}
	})

	t.Run("a review without findings keeps its lists", func(t *testing.T) {
		a := base()
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) { return answers(t), nil }
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if res.Findings == nil || res.Verifications == nil || res.Comments == nil || len(res.Comments) != 0 {
			t.Fatalf("a review without findings encodes a list as null: %+v", res)
		}
	})

	t.Run("experts and the judge are given the author's words", func(t *testing.T) {
		a, req := base(), request()
		req.Change.Description = "Expire sessions at the boundary\n\nA session was valid one second too long."
		var mu sync.Mutex
		var told [][]string
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			mu.Lock()
			told = append(told, append(slices.Clone(t.Commits), t.Description))
			mu.Unlock()
			return answers(t, finding(t.Group.Files[0], 3, t.Expert.Evidence[len(t.Expert.Evidence)-1])), nil
		}
		a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			mu.Lock()
			told = append(told, append(slices.Clone(t.Commits), t.Description))
			mu.Unlock()
			return verdicts(t, true, "holds"), nil
		}
		req.Verification = workflow.ModeReadOnly
		run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
		if len(told) < 3 || slices.ContainsFunc(told, func(c []string) bool {
			return !slices.Equal(c, append(slices.Clone(req.Change.Commits), req.Change.Description))
		}) {
			t.Fatalf("commit messages and description handed over: %v", told)
		}
	})

	t.Run("code in a ui or scripts group has a correctness reader", func(t *testing.T) {
		a := base()
		a.plan = func(workflow.PlanTask) (pulls.Analysis, error) {
			return pulls.Analysis{OverallSummary: "s", Groups: []pulls.DiffGroup{group("page", "ui", false, "app/session.py"), group("tool", "scripts", false, "app/login.py")}}, nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		for _, g := range res.Plan.Groups {
			if !slices.ContainsFunc(g.Experts, func(p workflow.Pick) bool { return p.Name == "correctness" }) {
				t.Fatalf("group %s has no correctness expert: %+v", g.Category, g.Experts)
			}
		}
	})

	t.Run("requested expert", func(t *testing.T) {
		a := base()
		req := request()
		req.Experts = []string{"security"}
		ws := &workspace{ci: workflow.Check{Passed: true}}
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
			return workflow.Report{Loaded: t.Skills, Findings: []workflow.Claim{{Skill: "golang-concurrency", Path: t.Group.Files[0], Line: 1, Severity: workflow.SeverityNote, Subject: "s", Evidence: workflow.EvidenceInspection}}}, nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if got := offered["correctness:app/session.py,app/login.py"]; len(got) != 1 || got[0] != "code-review" {
			t.Fatalf("a Go skill was offered for Python files: %v", got)
		}
		if len(res.Findings) != 0 || !strings.Contains(strings.Join(res.Uncertainty.Notes, " "), "golang-concurrency") {
			t.Fatalf("a finding citing a skill not offered was accepted: %+v, notes %v", res.Findings, res.Uncertainty.Notes)
		}
		if len(res.Uncertainty.Unread) != 1 || res.Uncertainty.Unread[0].Skill != "security-review" {
			t.Fatalf("unread %+v", res.Uncertainty.Unread)
		}
	})
}

func TestScenarioAndShort(t *testing.T) {
	task := workflow.ReviewTask{
		Expert: workflow.Expert{Name: "correctness", Evidence: []workflow.EvidenceKind{workflow.EvidenceTest, workflow.EvidenceInspection}},
		Group:  workflow.Group{Files: []string{"a.go"}},
	}
	bug := workflow.Claim{Skill: "other", Category: "c", Path: "a.go", Line: 1, Severity: workflow.SeverityError, Subject: "s", Evidence: workflow.EvidenceTest}
	if reasons := workflow.Validate(task, workflow.Report{Findings: []workflow.Claim{bug}}); len(reasons) != 1 || !strings.Contains(reasons[0], "scenario") {
		t.Fatalf("a bug without a scenario: %v", reasons)
	}
	bug.Evidence = workflow.EvidenceInspection
	if reasons := workflow.Validate(task, workflow.Report{Findings: []workflow.Claim{bug}}); len(reasons) != 0 {
		t.Fatalf("an argument needs no scenario: %v", reasons)
	}

	a := base()
	long := strings.Repeat("word ", 30)
	a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
		if t.Expert.Name == "correctness" {
			f := finding("app/session.py", 10, workflow.EvidenceInspection)
			f.Subject = long
			return answers(t, f), nil
		}
		return answers(t), nil
	}
	res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
	if c := res.Comments[0]; len([]rune(c.Short)) > 60 || !strings.HasSuffix(c.Short, "…") || c.Scenario == "" {
		t.Fatalf("short %q, scenario %q", c.Short, c.Scenario)
	}
}

func TestResume(t *testing.T) {
	ar := &archive{}
	first := base()
	want := run(t, first, &workspace{ci: workflow.Check{Passed: true}}, ar, request())

	again := base()
	got := run(t, again, &workspace{ci: workflow.Check{Passed: true}}, ar, request())
	if n := again.n(workflow.RolePlanner) + again.n(workflow.RoleExpert) + again.n(workflow.RoleFix); n != 0 {
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
		"no name":      {Tier: workflow.TierCheap, Evidence: []workflow.EvidenceKind{workflow.EvidenceInspection}},
		"bad tier":     {Name: "x", Tier: "huge", Evidence: []workflow.EvidenceKind{workflow.EvidenceInspection}},
		"other listed": {Name: "x", Tier: workflow.TierCheap, Skills: []workflow.Skill{{Name: "other"}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceInspection}},
		"bad glob":     {Name: "x", Tier: workflow.TierCheap, Skills: []workflow.Skill{{Name: "s", Files: []string{"[x"}}}, Evidence: []workflow.EvidenceKind{workflow.EvidenceInspection}},
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
		{Path: "api/a.go", Added: 1500, Diff: gitDiff("api/a.go", "a")},
		{Path: "api/b.go", Added: 900, Diff: gitDiff("api/b.go", "b")},
		{Path: "web/c.ts", Added: 300, Diff: gitDiff("web/c.ts", "c")},
		{Path: "web/d.ts", Added: 200, Diff: gitDiff("web/d.ts", "d")},
		{Path: "README.md", Added: 50, Diff: gitDiff("README.md", "r")},
		{Path: "data/huge.txt", Added: 10, Diff: "+" + strings.Repeat("x", 600<<10) + "\n"},
	}
	return req
}

func scopeAnalysis(t workflow.PlanTask) (pulls.Analysis, error) {
	g := group("all", "core", false)
	for _, f := range t.Diff.Files {
		g.FilePaths = append(g.FilePaths, f.Path)
	}
	return pulls.Analysis{OverallSummary: "a change across the repository", Groups: []pulls.DiffGroup{g}}, nil
}

func TestScopes(t *testing.T) {
	t.Run("a large change is split into scopes, each planned and reviewed on its own", func(t *testing.T) {
		a := base()
		a.plan = scopeAnalysis
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
		a.plan = func(t workflow.PlanTask) (pulls.Analysis, error) {
			if slices.ContainsFunc(t.Diff.Files, func(f pulls.FileChange) bool { return f.Path == "api/a.go" }) {
				return pulls.Analysis{OverallSummary: "s", Groups: []pulls.DiffGroup{}}, nil
			}
			return scopeAnalysis(t)
		}
		var reviewed [][]string
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			reviewed = append(reviewed, t.Group.Files)
			return answers(t), nil
		}
		req := large()
		req.Budget = workflow.Budget{Tokens: 534}
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

func TestReproductionCommand(t *testing.T) {
	a := base()
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
	run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
	if testOne != "pytest -k {test}" {
		t.Fatalf("expert got test_one %q", testOne)
	}
}

func TestParallel(t *testing.T) {
	files := make([]workflow.File, 6)
	groups := make([]pulls.DiffGroup, 6)
	panel := make([]workflow.Expert, 6)
	for i := range files {
		files[i] = workflow.File{Path: fmt.Sprintf("app/m%d.py", i), Added: 1, Diff: gitDiff(fmt.Sprintf("app/m%d.py", i), "x")}
		groups[i] = group(fmt.Sprintf("m%d", i), "security", false, files[i].Path)
		panel[i] = workflow.Expert{Name: fmt.Sprintf("e%d", i), Tier: workflow.TierMid, Evidence: []workflow.EvidenceKind{workflow.EvidenceInspection}}
	}
	setup := func(experts []string, budget int64) (*agents, *atomic.Int32, *workflow.Workflow, workflow.Request) {
		a := base()
		a.plan = func(workflow.PlanTask) (pulls.Analysis, error) {
			return pulls.Analysis{OverallSummary: "six modules", Groups: groups}, nil
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
		req.Budget, req.Experts = workflow.Budget{Tokens: budget}, experts
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
		a, _, w, req := setup(nil, 1_000_000)
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

	t.Run("one judge reads the findings of a group, and the groups' judges run at once", func(t *testing.T) {
		for name, tc := range map[string]struct {
			experts       []string
			lines         int
			judges, peak  int32
			verifications int
		}{
			"a judge for each group":           {nil, 1, 6, 4, 6},
			"a large group is read in batches": {[]string{"correctness"}, 3, 3, 3, 18},
		} {
			a, _, w, req := setup(tc.experts, 1_000_000)
			a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
				claims := make([]workflow.Claim, 0, len(t.Group.Files))
				for _, f := range t.Group.Files {
					for line := range tc.lines {
						claims = append(claims, finding(f, 1+10*line, workflow.EvidenceInspection))
					}
				}
				return answers(t, claims...), nil
			}
			var running, peak atomic.Int32
			a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
				n := running.Add(1)
				for p := peak.Load(); n > p && !peak.CompareAndSwap(p, n); p = peak.Load() {
				}
				time.Sleep(50 * time.Millisecond)
				running.Add(-1)
				return verdicts(t, true, "r"), nil
			}
			req.Verification = workflow.ModeReadOnly
			res := review(w, a, req)
			if int32(a.n(workflow.RoleJudge)) != tc.judges || peak.Load() != tc.peak || len(res.Verifications) != tc.verifications {
				t.Fatalf("%s: judges %d, peak %d, verifications %d", name, a.n(workflow.RoleJudge), peak.Load(), len(res.Verifications))
			}
		}
	})

	t.Run("a finding the judge leaves out stays unverified", func(t *testing.T) {
		a, _, w, req := setup([]string{"correctness"}, 1_000_000)
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			return answers(t, finding(t.Group.Files[0], 1, workflow.EvidenceInspection), finding(t.Group.Files[1], 1, workflow.EvidenceInspection)), nil
		}
		a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			return verdicts(t, false, "the code is right")[:1], nil
		}
		req.Verification = workflow.ModeReadOnly
		res := review(w, a, req)
		if len(res.Verifications) != 2 || res.Verifications[0].Status != workflow.StatusRejected || res.Verifications[1].Status != workflow.StatusUnverified || len(res.Uncertainty.Unverified) != 1 || len(res.Comments) != 2 || res.Comments[0].Objection != "" || res.Comments[1].Objection == "" {
			t.Fatalf("verifications %+v, comments %+v", res.Verifications, res.Comments)
		}
	})

	t.Run("a spent budget stops the experts still waiting", func(t *testing.T) {
		a, _, w, req := setup([]string{"e0", "e1", "e2", "e3", "e4", "e5"}, 100)
		res := review(w, a, req)
		if res.Outcome != workflow.OutcomePartial || a.n(workflow.RoleExpert) != 4 {
			t.Fatalf("outcome %s, experts %d", res.Outcome, a.n(workflow.RoleExpert))
		}
	})
}

func TestOrder(t *testing.T) {
	a := base()
	a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
		low := finding("app/session.py", 2, workflow.EvidenceInspection)
		low.Severity = workflow.SeverityNote
		switch t.Expert.Name {
		case "correctness":
			return answers(t, low, finding("app/session.py", 20, workflow.EvidenceInspection), finding("app/session.py", 40, workflow.EvidenceInspection), finding("app/session.py", 60, workflow.EvidenceInspection)), nil
		case "security":
			return answers(t, finding("app/session.py", 61, workflow.EvidenceAnalysis)), nil
		}
		return answers(t), nil
	}
	a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
		out := verdicts(t, true, "holds")
		for i, f := range t.Findings {
			if f.Line == 20 {
				out[i] = workflow.Judgement{Finding: f.ID, Valid: false, Reason: "the caller checks"}
			}
		}
		return out, nil
	}
	req := request()
	req.Verification = workflow.ModeReadOnly
	res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
	lines := make([]int, 0, len(res.Comments))
	for _, c := range res.Comments {
		lines = append(lines, c.Line)
	}
	if want := []int{60, 40, 20, 2}; !slices.Equal(lines, want) {
		t.Fatalf("comments at lines %v, want %v: two experts and stronger evidence first, then one expert, then what the judge disputes, then the nitpick", lines, want)
	}
}

func TestNeighbours(t *testing.T) {
	a := base()
	a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
		switch t.Expert.Name {
		case "correctness":
			return answers(t, finding("app/session.py", 20, workflow.EvidenceInspection), finding("app/session.py", 21, workflow.EvidenceInspection)), nil
		case "security":
			return answers(t, finding("app/session.py", 21, workflow.EvidenceAnalysis)), nil
		}
		return answers(t), nil
	}
	req := request()
	req.Verification = workflow.ModeNone
	res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
	experts := 0
	for _, c := range res.Comments {
		experts += len(c.Experts)
	}
	if len(res.Comments) != 2 || experts != 3 {
		t.Fatalf("%d comments from %d experts' findings, want 2 from 3: one expert's two findings are two problems, and another expert's beside them is one of the two", len(res.Comments), experts)
	}
}

func TestThoroughness(t *testing.T) {
	t.Run("a low finding is a nitpick nobody verifies", func(t *testing.T) {
		a := base()
		a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
			if t.Expert.Name != "correctness" {
				return answers(t), nil
			}
			return answers(t, workflow.Claim{Skill: "other", Category: "naming", Path: "app/session.py", Line: 2, Severity: workflow.SeverityNote, Subject: "rename x", Evidence: workflow.EvidenceInspection}), nil
		}
		req := request()
		req.Verification = workflow.ModeReadOnly
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
		if len(res.Comments) != 1 || res.Comments[0].Label != workflow.LabelNitpick || a.n(workflow.RoleJudge) != 0 || len(res.Verifications) != 0 {
			t.Fatalf("comments %+v, verifications %+v", res.Comments, res.Verifications)
		}
	})

	t.Run("a test the change breaks is a lead for the experts", func(t *testing.T) {
		for name, tc := range map[string]struct {
			ws   workspace
			lead bool
		}{
			"broken by the change":   {workspace{regressed: []string{"tests/test_login.py"}}, true},
			"failing before as well": {workspace{baseline: []string{"tests/test_login.py"}}, false},
			"green":                  {workspace{}, false},
		} {
			a := base()
			var mu sync.Mutex
			leads := map[string]*workflow.TestRun{}
			a.review = func(t workflow.ReviewTask) (workflow.Report, error) {
				mu.Lock()
				leads[t.Expert.Name] = t.Regression
				mu.Unlock()
				return answers(t), nil
			}
			tc.ws.ci = workflow.Check{Passed: true}
			res := run(t, a, &tc.ws, &archive{}, request())
			if (res.Regression != nil) != tc.lead || (leads["correctness"] != nil) != tc.lead {
				t.Fatalf("%s: regression %+v, leads %+v", name, res.Regression, leads)
			}
			if tc.lead && !slices.Equal(leads["correctness"].Failed, []string{"tests/test_login.py"}) {
				t.Fatalf("%s: lead %+v", name, leads["correctness"])
			}
		}
	})

	t.Run("a risky path makes its group core", func(t *testing.T) {
		a := base()
		a.plan = func(workflow.PlanTask) (pulls.Analysis, error) {
			p, err := goodAnalysis(workflow.PlanTask{})
			p.Groups[0].Critical = false
			return p, err
		}
		req := request()
		req.Project.Risk = []string{"app/session.py"}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, req)
		if !res.Plan.Groups[0].Core || res.Plan.Groups[1].Core {
			t.Fatalf("groups %+v", res.Plan.Groups)
		}
	})
}
