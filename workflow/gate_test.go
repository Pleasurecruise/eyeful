package workflow_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Pleasurecruise/eyeful/workflow"
)

func fixed(edits []workflow.Edit) bool {
	return slices.ContainsFunc(edits, func(e workflow.Edit) bool { return e.Old == "bug" })
}

func TestGates(t *testing.T) {
	broken := errors.New("the runner died")
	for name, tc := range map[string]struct {
		fail   func([]workflow.Edit, workflow.Target) error
		fix    func(workflow.FixTask) ([]workflow.Edit, error)
		status workflow.Status
		reason string
	}{
		"the test cannot run on the change": {
			fail: func(e []workflow.Edit, t workflow.Target) error {
				if len(t.Tests) > 0 && !t.Base && !fixed(e) {
					return broken
				}
				return nil
			},
			status: workflow.StatusUnverified, reason: "the reproduction could not run: the runner died",
		},
		"the test cannot run on the base": {
			fail: func(_ []workflow.Edit, t workflow.Target) error {
				if len(t.Tests) > 0 && t.Base {
					return broken
				}
				return nil
			},
			status: workflow.StatusUnverified, reason: "the reproduction could not run on the base commit: the runner died",
		},
		"no fix is written": {
			fix:    func(workflow.FixTask) ([]workflow.Edit, error) { return nil, broken },
			status: workflow.StatusReproduced, reason: "no fix: the runner died",
		},
		"the fix leaves the test failing": {
			fix: func(workflow.FixTask) ([]workflow.Edit, error) {
				return []workflow.Edit{{Path: "app/session.py", Old: "same", New: "other"}}, nil
			},
			status: workflow.StatusReproduced, reason: "the fix does not make the test pass",
		},
		"the fixed test cannot run": {
			fail: func(e []workflow.Edit, t workflow.Target) error {
				if len(t.Tests) > 0 && fixed(e) {
					return broken
				}
				return nil
			},
			status: workflow.StatusReproduced, reason: "the fixed test could not run: the runner died",
		},
		"the suite cannot run with the fix": {
			fail: func(e []workflow.Edit, t workflow.Target) error {
				if len(t.Tests) == 0 && fixed(e) {
					return broken
				}
				return nil
			},
			status: workflow.StatusReproduced, reason: "the suite could not run with the fix: the runner died",
		},
		"the suite cannot run on the change": {
			fail: func(e []workflow.Edit, t workflow.Target) error {
				if len(t.Tests) == 0 && !fixed(e) {
					return broken
				}
				return nil
			},
			status: workflow.StatusReproduced, reason: "the suite could not run on the change: the runner died",
		},
	} {
		a := base()
		if tc.fix != nil {
			a.fix = tc.fix
		}
		var tried []workflow.Verification
		a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			tried = append(tried, t.Tried...)
			return verdicts(t, true, "read"), nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}, fail: tc.fail}, &archive{}, request())
		v := res.Verifications[0]
		if tc.status == workflow.StatusUnverified {
			if len(tried) != 1 {
				t.Fatalf("%s: the judge was not handed the tried reproduction: %+v", name, res.Verifications)
			}
			v = tried[0]
		}
		if v.Status != tc.status || v.Reason != tc.reason {
			t.Errorf("%s: status %s, reason %q", name, v.Status, v.Reason)
		}
		if tc.status == workflow.StatusReproduced && (len(res.Comments) != 1 || res.Comments[0].Fix != nil) {
			t.Errorf("%s: an unverified fix reached the comment: %+v", name, res.Comments)
		}
	}
}

func TestRepro(t *testing.T) {
	good := *repro(workflow.OracleSpecified, "real")
	for want, edit := range map[string]func(*workflow.Repro){
		"it names no test to run":                      func(r *workflow.Repro) { r.Run = "" },
		"it needs fails":                               func(r *workflow.Repro) { r.Fails = " " },
		`oracle "guess" is not`:                        func(r *workflow.Repro) { r.Oracle = "guess" },
		`oracle "specified" needs the quoted text`:     func(r *workflow.Repro) { r.Quote = "" },
		"fails is not in the test":                     func(r *workflow.Repro) { r.Fails = "another message" },
		`only oracle "derived" can name a test`:        func(r *workflow.Repro) { r.Test = workflow.Edit{} },
		"the reproduction test is a new file":          func(r *workflow.Repro) { r.Test.Old = "x" },
		"give its path, leave old empty":               func(r *workflow.Repro) { r.Test.Path = "" },
		"oracle \"implicit\" hands in a new test file": func(r *workflow.Repro) { r.Oracle, r.Test = workflow.OracleImplicit, workflow.Edit{} },
	} {
		a := base()
		bad := good
		edit(&bad)
		a.retest = func(workflow.RetestTask) (workflow.Repro, error) { return bad, nil }
		var tried []workflow.Verification
		a.judge = func(t workflow.JudgeTask) ([]workflow.Judgement, error) {
			tried = append(tried, t.Tried...)
			return verdicts(t, false, "read"), nil
		}
		res := run(t, a, &workspace{ci: workflow.Check{Passed: true}}, &archive{}, request())
		if len(tried) != 1 || tried[0].Repro != nil || !strings.HasPrefix(tried[0].Reason, "the reproduction handed in cannot be used: ") || !strings.Contains(tried[0].Reason, want) {
			t.Errorf("%s: tried %+v", want, tried)
		}
		if a.n(workflow.RoleFix) != 0 || res.Verifications[0].Status != workflow.StatusRejected {
			t.Errorf("%s: fixes %d, verifications %+v", want, a.n(workflow.RoleFix), res.Verifications)
		}
	}
}
