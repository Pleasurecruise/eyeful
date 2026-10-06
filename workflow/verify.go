package workflow

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
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
	weak := func(f Finding) int {
		if oracleStrength[f.Repro.Oracle] == StrengthStrong {
			return 0
		}
		return 1
	}
	slices.SortStableFunc(out, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(weak(a), weak(b)),
			-cmp.Compare(severityRank[a.Severity], severityRank[b.Severity]),
			-cmp.Compare(corroboration(a, findings), corroboration(b, findings)),
		)
	})
	return out
}

func (w *Workflow) reproduce(ctx context.Context, s Session, l *ledger, f Finding, files map[string]File, target Target, baseline func() (TestRun, error)) (verifyState, error) {
	st := verifyState{Verification: Verification{Finding: f.ID, Status: StatusUnverified}, Runs: []Run{}}
	v := &st.Verification
	test := []Edit{f.Repro.Test}
	run, err := s.Workspace.Test(ctx, test, Target{Tests: []string{f.Repro.Run}})
	switch {
	case err != nil:
		v.Reason = "the reproduction could not run: " + err.Error()
		return st, nil
	case run.Passed:
		v.Status = StatusNotReproduced
		v.Reason = "the test passes on the change"
		return st, nil
	}
	v.Status = StatusReproduced
	if !l.allow() {
		v.Reason = "budget exhausted before a fix was written"
		return st, nil
	}
	blind := f
	blind.Discussion = ""
	fix, r, err := invoke(ctx, l, Run{Role: RoleFix, Expert: f.Expert}, s.Agents.Fix, FixTask{Finding: blind, Files: pick(files, []string{f.Path, f.Repro.Test.Path})})
	if err != nil {
		return st, err
	}
	st.Runs = append(st.Runs, r)
	if r.Error != "" {
		v.Reason = "no fix: " + r.Error
		return st, nil
	}
	v.Fix = fix
	fixed := append(slices.Clone(test), fix...)
	if run, err = s.Workspace.Test(ctx, fixed, Target{Tests: []string{f.Repro.Run}}); err != nil || !run.Passed {
		v.Reason = "the fix does not make the test pass"
		if err != nil {
			v.Reason = "the fixed test could not run: " + err.Error()
		}
		return st, nil
	}
	before, err := baseline()
	if err != nil {
		v.Reason = "the suite could not run on the change: " + err.Error()
		return st, nil
	}
	after, err := s.Workspace.Test(ctx, fixed, target)
	if err != nil {
		v.Reason = "the suite could not run with the fix: " + err.Error()
		return st, nil
	}
	var broken []string
	for _, t := range after.Failed {
		if !slices.Contains(before.Failed, t) {
			broken = append(broken, t)
		}
	}
	if len(broken) > 0 {
		v.Reason = fmt.Sprintf("the fix breaks %v", broken)
		return st, nil
	}
	v.Status = StatusFixed
	return st, nil
}

func (w *Workflow) judge(ctx context.Context, s Session, l *ledger, f Finding, files map[string]File) (verifyState, error) {
	st := verifyState{Verification: Verification{Finding: f.ID, Status: StatusUnverified}, Runs: []Run{}}
	if !l.allow() {
		st.Verification.Reason = "budget exhausted before the judge ran"
		return st, nil
	}
	j, r, err := invoke(ctx, l, Run{Role: RoleJudge, Expert: f.Expert}, s.Agents.Judge, JudgeTask{Finding: f, Files: pick(files, []string{f.Path})})
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

func (w *Workflow) baseline(ctx context.Context, s Session, l *ledger, target Target) func() (TestRun, error) {
	return sync.OnceValues(func() (TestRun, error) {
		st, err := checkpoint(ctx, s.Archive, l, "baseline", func() (baselineState, error) {
			run, err := s.Workspace.Test(ctx, nil, target)
			st := baselineState{Run: run}
			if err != nil {
				st.Error = err.Error()
			}
			return st, nil
		})
		if err == nil && st.Error != "" {
			err = errors.New(st.Error)
		}
		return st.Run, err
	})
}

func (w *Workflow) verify(ctx context.Context, s Session, l *ledger, req Request, level Level, setupErr string, findings []Finding, files map[string]File) ([]Verification, error) {
	out := []Verification{}
	var queue []Finding
	switch req.Verification {
	case ModeNone:
		return out, nil
	case ModeReadOnly:
		queue = findings
	case ModeExecution:
		for _, f := range findings {
			if f.Repro != nil {
				queue = append(queue, f)
			}
		}
		queue = verificationOrder(queue)
	}
	if req.Verification == ModeReadOnly {
		verdicts := make([]Verification, len(queue))
		errs := make([]error, len(queue))
		var wg sync.WaitGroup
		turns := make(chan struct{}, maxParallel)
		for i, f := range queue {
			wg.Go(func() {
				select {
				case turns <- struct{}{}:
				case <-ctx.Done():
					errs[i] = fmt.Errorf("wait to judge %s: %w", f.ID, ctx.Err())
					return
				}
				defer func() { <-turns }()
				st, err := checkpoint(ctx, s.Archive, l, "verify/"+f.ID, func() (verifyState, error) { return w.judge(ctx, s, l, f, files) })
				verdicts[i], errs[i] = st.Verification, err
			})
		}
		wg.Wait()
		return append(out, verdicts...), errors.Join(errs...)
	}
	target := Target{Touched: slices.Sorted(maps.Keys(files)), Full: levels[level].full}
	baseline := w.baseline(ctx, s, l, target)
	for _, f := range queue {
		reason := ""
		switch {
		case !levels[level].verify:
			reason = fmt.Sprintf("the %s level does not verify", level)
		case setupErr != "":
			reason = "setup failed: " + setupErr
		}
		if reason != "" {
			out = append(out, Verification{Finding: f.ID, Status: StatusUnverified, Reason: reason})
			continue
		}
		st, err := checkpoint(ctx, s.Archive, l, "verify/"+f.ID, func() (verifyState, error) {
			return w.reproduce(ctx, s, l, f, files, target, baseline)
		})
		if err != nil {
			return nil, err
		}
		out = append(out, st.Verification)
	}
	return out, nil
}
