package workflow

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
)

var (
	severityRank = map[Severity]int{SeverityError: 3, SeverityWarning: 2, SeverityNote: 1}
	crash        = regexp.MustCompile(`panic:|panicked at|fatal error:|DATA RACE|Sanitizer|SIG[A-Z]+|[Ss]egmentation fault|timed out|\w(Error|Exception)\b`)
	notCrash     = regexp.MustCompile(`(?i)assert|import|modulenotfound|syntax`)
)

func corroboration(f Finding, all []Finding, root map[string]string) int {
	// TODO(workflow): weigh corroboration across model families above agreement within one.
	experts := map[string]bool{}
	for _, o := range all {
		if root[o.ID] == root[f.ID] {
			experts[o.Expert] = true
		}
	}
	return len(experts)
}

func verificationOrder(findings []Finding, root map[string]string) []Finding {
	out := slices.Clone(findings)
	slices.SortStableFunc(out, func(a, b Finding) int {
		return cmp.Or(
			-cmp.Compare(severityRank[a.Severity], severityRank[b.Severity]),
			-cmp.Compare(corroboration(a, findings, root), corroboration(b, findings, root)),
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
	case !slices.Contains([]Oracle{OracleSpecified, OracleDerived, OracleImplicit, OracleHuman}, r.Oracle):
		return fmt.Sprintf("oracle %q is not specified, derived, implicit or human", r.Oracle)
	case r.Oracle == OracleSpecified && r.Quote == "":
		return fmt.Sprintf("oracle %q needs the quoted text", OracleSpecified)
	case r.Oracle != OracleImplicit && r.Test != Edit{} && !strings.Contains(r.Test.New, r.Fails):
		return fmt.Sprintf("fails is not in the test: unless the oracle is %q, fails is a message the test itself writes, such as its assertion message", OracleImplicit)
	case r.Oracle != OracleDerived && r.Test == Edit{}:
		return fmt.Sprintf("only oracle %q can name a test already in the repository in run and leave test out; oracle %q hands in a new test file", OracleDerived, r.Oracle)
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

func grade(ctx context.Context, s Session, r *Repro, change Change, base TestStatus) Oracle {
	held := r.Oracle != OracleHuman
	switch {
	case r.Oracle == OracleImplicit:
		held = !strings.Contains(r.Test.New, r.Fails) && crash.MatchString(r.Fails) && !notCrash.MatchString(r.Fails)
	case r.Oracle == OracleDerived && r.Test != Edit{}:
		held = base == TestPassed
	case r.Oracle == OracleSpecified:
		text := strings.Join(append(slices.Clone(change.Commits), change.Description), "\n")
		if r.Source != "" {
			text = ""
			if read, err := s.Workspace.Read(ctx, r.Source); err == nil && matchAny(docsGlobs, r.Source) {
				text = read
			}
		}
		held = len(strings.Fields(r.Quote)) >= quoteWords && strings.Contains(strings.Join(strings.Fields(text), " "), strings.Join(strings.Fields(r.Quote), " "))
	}
	switch {
	case held:
		return r.Oracle
	case base == TestPassed:
		return OracleDerived
	}
	return OracleHuman
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
	if runErr == nil && len(seen) == reproRuns && seen[0] == TestError {
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
	graded.Oracle = grade(ctx, s, v.Repro, req.Change, v.Base)
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
	after, err := s.Workspace.Test(ctx, fixed, Target{})
	if err != nil {
		return stop("the suite could not run with the fix: " + err.Error())
	}
	if !after.Passed && len(after.Failed) == 0 {
		return stop("the suite fails with the fix, and its command does not name the tests that fail")
	}
	broken := slices.DeleteFunc(slices.Clone(after.Failed), func(t string) bool { return slices.Contains(suite.Change.Failed, t) })
	if len(broken) > 0 {
		return stop(fmt.Sprintf("the fix breaks %v", broken))
	}
	v.Status = StatusFixed
	return st, nil
}

func (w *Workflow) judge(ctx context.Context, s Session, l *ledger, batch []Finding, tried map[string]*Verification, files map[string]File, change Change) (judgeState, error) {
	st := judgeState{Verifications: make([]Verification, len(batch)), Runs: []Run{}}
	task := JudgeTask{Findings: batch, Commits: change.Commits, Description: change.Description}
	var paths []string
	for i, f := range batch {
		st.Verifications[i] = Verification{Finding: f.ID, Status: StatusUnverified}
		if t := tried[f.ID]; t != nil {
			st.Verifications[i].Repro, st.Verifications[i].Change = t.Repro, t.Change
			task.Tried = append(task.Tried, *t)
		}
		if !slices.Contains(paths, f.Path) {
			paths = append(paths, f.Path)
		}
	}
	task.Files = pick(files, paths)
	stop := func(reason string) (judgeState, error) {
		for i := range st.Verifications {
			st.Verifications[i].Reason = reason
		}
		return st, nil
	}
	answers, r, err := invoke(ctx, l, Run{Role: RoleJudge}, s.Agents.Judge, task)
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	if r.Error != "" {
		return stop("the judge failed: " + r.Error)
	}
	for i := range st.Verifications {
		v := &st.Verifications[i]
		j := slices.IndexFunc(answers, func(j Judgement) bool { return j.Finding == v.Finding })
		if j < 0 {
			v.Reason = "the judge gave no verdict on this finding"
			continue
		}
		v.Status, v.Reason = StatusRejected, answers[j].Reason
		if answers[j].Valid {
			v.Status = StatusConfirmed
		}
	}
	return st, nil
}

func each[T Finding | []Finding, S verifyState | judgeState](ctx context.Context, queue []T, fn func(T) (S, error)) ([]S, error) {
	states := make([]S, len(queue))
	errs := make([]error, len(queue))
	var wg sync.WaitGroup
	turns := make(chan struct{}, maxParallel)
	for i, item := range queue {
		wg.Go(func() {
			select {
			case turns <- struct{}{}:
			case <-ctx.Done():
				errs[i] = fmt.Errorf("wait to check %d of %d: %w", i+1, len(queue), ctx.Err())
				return
			}
			defer func() { <-turns }()
			states[i], errs[i] = fn(item)
		})
	}
	wg.Wait()
	return states, errors.Join(errs...)
}

func cluster(findings []Finding, root map[string]string) ([]Finding, map[string][]Finding) {
	var leads []Finding
	beside := map[string][]Finding{}
	for _, f := range verificationOrder(findings, root) {
		if i := slices.IndexFunc(leads, func(lead Finding) bool { return root[lead.ID] == root[f.ID] }); i >= 0 {
			beside[leads[i].ID] = append(beside[leads[i].ID], f)
			continue
		}
		leads = append(leads, f)
	}
	return leads, beside
}

func (w *Workflow) execute(ctx context.Context, s Session, l *ledger, req Request, suite suiteState, findings []Finding, files map[string]File, root map[string]string) ([]Verification, []Finding, map[string]*Verification, error) {
	leads, beside := cluster(findings, root)
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

func (w *Workflow) verify(ctx context.Context, s Session, l *ledger, req Request, execution bool, suite suiteState, findings []Finding, files map[string]File, root map[string]string) ([]Verification, error) {
	if req.Verification == ModeNone {
		return []Verification{}, nil
	}
	var run, read []Finding
	for _, f := range findings {
		switch {
		case f.Severity == SeverityNote:
		case execution && f.Evidence == EvidenceTest && req.Project.TestOne != "":
			run = append(run, f)
		default:
			read = append(read, f)
		}
	}
	out, unshown, tried, err := w.execute(ctx, s, l, req, suite, run, files, root)
	if err != nil {
		return nil, err
	}
	judged := slices.Concat(read, unshown)
	slices.SortStableFunc(judged, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.Group, b.Group), cmp.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	var batches [][]Finding
	for _, f := range judged {
		if n := len(batches); n == 0 || len(batches[n-1]) == judgeBatch || batches[n-1][0].Group != f.Group {
			batches = append(batches, nil)
		}
		batches[len(batches)-1] = append(batches[len(batches)-1], f)
	}
	states, err := each(ctx, batches, func(b []Finding) (judgeState, error) {
		var members strings.Builder
		for _, f := range b {
			members.WriteString(f.ID + "\n")
		}
		key := sha256.Sum256([]byte(members.String()))
		return checkpoint(ctx, s.Archive, l, fmt.Sprintf("judge/%s/%x", req.Verification, key[:8]), func() (judgeState, error) { return w.judge(ctx, s, l, b, tried, files, req.Change) })
	})
	for _, st := range states {
		out = append(out, st.Verifications...)
	}
	return out, err
}
