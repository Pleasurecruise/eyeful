package workflow

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
)

var severityRank = map[Severity]int{SeverityCritical: 4, SeverityHigh: 3, SeverityMedium: 2, SeverityLow: 1}

func corroboration(f Finding, all []Finding) int {
	// TODO(workflow): weigh corroboration across model families above agreement within one.
	experts := map[string]bool{}
	for _, o := range all {
		if near(f, o) {
			experts[o.Expert] = true
		}
	}
	return len(experts)
}

func verificationOrder(findings []Finding) []Finding {
	out := slices.Clone(findings)
	slices.SortStableFunc(out, func(a, b Finding) int {
		return cmp.Or(
			-cmp.Compare(severityRank[a.Severity], severityRank[b.Severity]),
			-cmp.Compare(corroboration(a, findings), corroboration(b, findings)),
		)
	})
	return out
}

func checkRepro(r Repro) string {
	switch {
	case r.Run == "":
		return "it names no test to run"
	case strings.TrimSpace(r.Fails) == "":
		return "it needs fails: the text the test prints when it fails as the finding claims"
	case !slices.Contains([]Oracle{OracleImplicit, OracleExisting, OracleSpec, OracleBase, OracleAgent}, r.Oracle):
		return fmt.Sprintf("oracle %q is not implicit, existing, spec, base or agent", r.Oracle)
	case r.Oracle == OracleSpec && r.Quote == "":
		return fmt.Sprintf("oracle %q needs the quoted text", OracleSpec)
	case (r.Oracle == OracleExisting) != (r.Test == Edit{}):
		return fmt.Sprintf("oracle %q names a test already in the repository in run and leaves test out; every other oracle hands in a new test file", OracleExisting)
	case r.Test.Old != "" || (r.Test.Path == "") != (r.Test.New == ""):
		return "the reproduction test is a new file: give its path, leave old empty and put the whole file in new"
	}
	return ""
}

func (r *Repro) edits() []Edit {
	if r.Test == (Edit{}) {
		return nil
	}
	return []Edit{r.Test}
}

func (r *Repro) status(run TestRun) TestStatus {
	switch {
	case run.Passed:
		return TestPassed
	case strings.Contains(run.Output, r.Fails):
		return TestFailed
	}
	return TestError
}

func grade(ctx context.Context, s Session, r *Repro, commits []string, base TestStatus) Oracle {
	oracle := r.Oracle
	switch {
	case oracle == OracleImplicit && strings.Contains(r.Test.New, r.Fails):
		oracle = OracleAgent
	case oracle == OracleBase && base != TestPassed:
		oracle = OracleAgent
	case oracle == OracleSpec:
		text := strings.Join(commits, "\n")
		if r.Source != "" {
			var err error
			if text, err = s.Workspace.Read(ctx, r.Source); err != nil {
				text = ""
			}
		}
		if !strings.Contains(strings.Join(strings.Fields(text), " "), strings.Join(strings.Fields(r.Quote), " ")) {
			oracle = OracleAgent
		}
	}
	if oracle == OracleAgent && base == TestPassed {
		return OracleBase
	}
	return oracle
}

func trial(ctx context.Context, s Session, r *Repro) ([]TestStatus, string, error) {
	var seen []TestStatus
	var output string
	for range reproRuns {
		run, err := s.Workspace.Test(ctx, r.edits(), Target{Tests: []string{r.Run}})
		if err != nil {
			return nil, output, err
		}
		if len(seen) == 0 {
			output = run.Output
		}
		seen = append(seen, r.status(run))
		if seen[len(seen)-1] != seen[0] {
			break
		}
	}
	return seen, output, nil
}

func (w *Workflow) write(ctx context.Context, s Session, l *ledger, f Finding, command string) (verifyState, error) {
	st := verifyState{Verification: Verification{Finding: f.ID, Status: StatusUnverified}, Runs: []Run{}}
	if !l.allow() {
		st.Verification.Reason = "budget exhausted before the reproduction was written"
		return st, nil
	}
	repro, r, err := invoke(ctx, l, Run{Role: RoleRepro, Expert: f.Expert}, s.Agents.Retest, RetestTask{Finding: f, Command: command})
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	switch problem := checkRepro(repro); {
	case r.Error != "":
		st.Verification.Reason = "no reproduction was written: " + r.Error
	case problem != "":
		st.Verification.Reason = "the reproduction handed in cannot be used: " + problem
	default:
		st.Verification.Repro = &repro
	}
	return st, nil
}

func (w *Workflow) reproduce(ctx context.Context, s Session, l *ledger, f Finding, repro Repro, req Request, files map[string]File, suite suiteState) (verifyState, error) {
	st := verifyState{Verification: Verification{Finding: f.ID, Status: StatusUnverified, Repro: &repro}, Runs: []Run{}}
	v := &st.Verification
	seen, output, runErr := trial(ctx, s, v.Repro)
	if runErr == nil && len(seen) == reproRuns && seen[0] == TestError && l.allow() {
		again, r, err := invoke(ctx, l, Run{Role: RoleRepro, Expert: f.Expert}, s.Agents.Retest, RetestTask{Finding: f, Command: req.Project.TestOne, Repro: v.Repro, Output: output})
		if err != nil {
			return st, err
		}
		st.Runs = append(st.Runs, r)
		if r.Error == "" && checkRepro(again) == "" {
			v.Repro = &again
			seen, _, runErr = trial(ctx, s, v.Repro)
		}
	}
	if runErr != nil {
		v.Reason = "the reproduction could not run: " + runErr.Error()
		return st, nil
	}
	v.Change = seen
	switch first := seen[0]; {
	case seen[len(seen)-1] != first:
		v.Reason = fmt.Sprintf("the reproduction is flaky: run %d of %d did not end like the first", len(seen), reproRuns)
		return st, nil
	case first == TestPassed:
		v.Status, v.Reason = StatusNotReproduced, "the test passes on the change"
		return st, nil
	case first == TestError:
		v.Reason = fmt.Sprintf("the test fails on the change, but its output does not contain %q", v.Repro.Fails)
		return st, nil
	}
	old, err := s.Workspace.Test(ctx, v.Repro.edits(), Target{Tests: []string{v.Repro.Run}, Base: true})
	if err != nil {
		v.Reason = "the reproduction could not run on the base commit: " + err.Error()
		return st, nil
	}
	if v.Base = v.Repro.status(old); v.Base == TestFailed {
		v.Status, v.Reason = StatusPreExisting, "the test fails the same way on the base commit"
		return st, nil
	}
	graded := *v.Repro
	graded.Oracle = grade(ctx, s, v.Repro, req.Change.Commits, v.Base)
	v.Status, v.Repro = StatusReproduced, &graded
	return w.repair(ctx, s, l, f, files, suite, st)
}

func (w *Workflow) repair(ctx context.Context, s Session, l *ledger, f Finding, files map[string]File, suite suiteState, st verifyState) (verifyState, error) {
	v := &st.Verification
	stop := func(reason string) (verifyState, error) {
		v.Reason = reason
		return st, nil
	}
	repro := v.Repro
	test := repro.edits()
	if !l.allow() {
		return stop("budget exhausted before a fix was written")
	}
	blind := f
	blind.Discussion = ""
	fix, r, err := invoke(ctx, l, Run{Role: RoleFix, Expert: f.Expert}, s.Agents.Fix, FixTask{Finding: blind, Repro: *repro, Files: pick(files, []string{f.Path, repro.Test.Path})})
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	if r.Error != "" {
		return stop("no fix: " + r.Error)
	}
	v.Fix = fix
	if len(test) > 0 && slices.ContainsFunc(fix, func(e Edit) bool { return path.Clean(e.Path) == path.Clean(repro.Test.Path) }) {
		return stop("the fix edits the reproduction test")
	}
	fixed := append(slices.Clone(test), fix...)
	run, err := s.Workspace.Test(ctx, fixed, Target{Tests: []string{repro.Run}})
	if err != nil {
		return stop("the fixed test could not run: " + err.Error())
	}
	if v.Fixed = repro.status(run); v.Fixed != TestPassed {
		return stop("the fix does not make the test pass")
	}
	if suite.Error != "" {
		return stop("the suite could not run on the change: " + suite.Error)
	}
	after, err := s.Workspace.Test(ctx, fixed, Target{Touched: slices.Sorted(maps.Keys(files))})
	if err != nil {
		return stop("the suite could not run with the fix: " + err.Error())
	}
	broken := slices.DeleteFunc(slices.Clone(after.Failed), func(t string) bool { return slices.Contains(suite.Change.Failed, t) })
	if len(broken) > 0 {
		return stop(fmt.Sprintf("the fix breaks %v", broken))
	}
	v.Status = StatusFixed
	return st, nil
}

func (w *Workflow) judge(ctx context.Context, s Session, l *ledger, f Finding, tried *Verification, files map[string]File) (verifyState, error) {
	st := verifyState{Verification: Verification{Finding: f.ID, Status: StatusUnverified}, Runs: []Run{}}
	if tried != nil {
		st.Verification.Repro, st.Verification.Change = tried.Repro, tried.Change
	}
	if !l.allow() {
		st.Verification.Reason = "budget exhausted before the judge ran"
		return st, nil
	}
	j, r, err := invoke(ctx, l, Run{Role: RoleJudge, Expert: f.Expert}, s.Agents.Judge, JudgeTask{Finding: f, Tried: tried, Files: pick(files, []string{f.Path})})
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	if r.Error != "" {
		st.Verification.Reason = "the judge failed: " + r.Error
		return st, nil
	}
	st.Verification.Status = StatusRejected
	if j.Valid {
		st.Verification.Status = StatusConfirmed
	}
	st.Verification.Reason = j.Reason
	return st, nil
}

func each(ctx context.Context, queue []Finding, fn func(Finding) (verifyState, error)) ([]verifyState, error) {
	states := make([]verifyState, len(queue))
	errs := make([]error, len(queue))
	var wg sync.WaitGroup
	turns := make(chan struct{}, maxParallel)
	for i, f := range queue {
		wg.Go(func() {
			select {
			case turns <- struct{}{}:
			case <-ctx.Done():
				errs[i] = fmt.Errorf("wait to check %s: %w", f.ID, ctx.Err())
				return
			}
			defer func() { <-turns }()
			states[i], errs[i] = fn(f)
		})
	}
	wg.Wait()
	return states, errors.Join(errs...)
}

func cluster(findings []Finding) ([]Finding, map[string][]Finding) {
	var leads []Finding
	beside := map[string][]Finding{}
	for _, f := range verificationOrder(findings) {
		if i := slices.IndexFunc(leads, func(lead Finding) bool { return near(lead, f) }); i >= 0 {
			beside[leads[i].ID] = append(beside[leads[i].ID], f)
			continue
		}
		leads = append(leads, f)
	}
	return leads, beside
}

func (w *Workflow) execute(ctx context.Context, s Session, l *ledger, req Request, suite suiteState, findings []Finding, files map[string]File) ([]Verification, []Finding, map[string]*Verification, error) {
	leads, beside := cluster(findings)
	written, err := each(ctx, leads, func(f Finding) (verifyState, error) {
		return checkpoint(ctx, s.Archive, l, "repro/"+f.ID, func() (verifyState, error) { return w.write(ctx, s, l, f, req.Project.TestOne) })
	})
	if err != nil {
		return nil, nil, nil, err
	}
	out := []Verification{}
	var unshown []Finding
	tried := map[string]*Verification{}
	for i, f := range leads {
		v := written[i].Verification
		if v.Repro != nil {
			st, err := checkpoint(ctx, s.Archive, l, "verify/execution/"+f.ID, func() (verifyState, error) {
				return w.reproduce(ctx, s, l, f, *v.Repro, req, files, suite)
			})
			if err != nil {
				return nil, nil, nil, err
			}
			v = st.Verification
		}
		if v.Status == StatusUnverified || v.Status == StatusNotReproduced {
			tried[f.ID] = &v
			unshown = append(append(unshown, f), beside[f.ID]...)
			continue
		}
		out = append(out, v)
	}
	return out, unshown, tried, nil
}

func (w *Workflow) verify(ctx context.Context, s Session, l *ledger, req Request, execution bool, suite suiteState, findings []Finding, files map[string]File) ([]Verification, error) {
	if req.Verification == ModeNone {
		return []Verification{}, nil
	}
	var run, read []Finding
	for _, f := range findings {
		switch {
		case f.Severity == SeverityLow:
		case execution && f.Evidence == EvidenceReproTest && req.Project.TestOne != "":
			run = append(run, f)
		default:
			read = append(read, f)
		}
	}
	out, unshown, tried, err := w.execute(ctx, s, l, req, suite, run, files)
	if err != nil {
		return nil, err
	}
	judge := func(f Finding) (verifyState, error) {
		return checkpoint(ctx, s.Archive, l, fmt.Sprintf("verify/%s/judge/%s", req.Verification, f.ID), func() (verifyState, error) { return w.judge(ctx, s, l, f, tried[f.ID], files) })
	}
	leads, beside := cluster(append(read, unshown...))
	first, err := each(ctx, leads, judge)
	if err != nil {
		return nil, err
	}
	var rest []Finding
	for i, st := range first {
		out = append(out, st.Verification)
		if st.Verification.Status != StatusConfirmed {
			rest = append(rest, beside[leads[i].ID]...)
		}
	}
	second, err := each(ctx, rest, judge)
	for _, st := range second {
		out = append(out, st.Verification)
	}
	return out, err
}
