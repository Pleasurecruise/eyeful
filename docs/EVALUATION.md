# Evaluation

This page defines how eyeful is evaluated. The scripts in `eval/` are built; most SWE-bench
subjects, the runs and the labelling are still to do ([Roadmap](ROADMAP.md)). Results will be added
once the main experiment has run.

## Research questions {#research-question}

Does planned multi-expert review with execution-based verification find more of the defects that
matter, with less noise and with evidence a maintainer can check, than simpler LLM reviewers, and
at what cost?

| Question           | Asks                                                                                                                |
| ------------------ | ------------------------------------------------------------------------------------------------------------------- |
| RQ1, detection     | Does a planned panel find more known defects than a single agent with the same model and skills?                    |
| RQ2, noise         | What proportion of reported findings is actionable under each verification condition?                               |
| RQ3, verifiability | How many findings can execution reproduce and fix, how many real defects does it fail to confirm, and at what cost? |

## Datasets

A subject is one change whose defects are known. Every subject comes from a public benchmark, and
only SWE-bench has tests that run, so RQ3 rests on it alone.

| Dataset                      | Subjects                                                        | Known defects                          | Answers       |
| ---------------------------- | --------------------------------------------------------------- | -------------------------------------- | ------------- |
| SWE-bench Verified, reverted | About 30 drawn from 293 tasks of seven Python projects; 1 built | The lines the reference fix touches    | RQ1, RQ2, RQ3 |
| Code Review Bench, offline   | 50 pull requests of five projects                               | 173 comments human reviewers confirmed | RQ1, RQ2      |
| ReviewBench                  | 219 pull requests, 187 repositories, 19 languages               | The benchmark's golden findings        | RQ1, RQ2      |

- A SWE-bench subject puts the reference fix in the base and takes it out in the change. It is
  built from the task's Docker image under a neutral name, with the issue text withheld, and its
  tests run in a fresh container without a network. The seven projects are those `subjects.py` has
  a test runner for.
- ReviewBench's 25 test pull requests are the ones eyeful is developed on. A held-out part, drawn
  from the rest by hash, checks a change once; the whole set is for the final run.
- The datasets live under `.eval/`, which git ignores. A subject's description, which names the
  defect, is kept apart from its repository, and every review runs in a sandbox that reads only
  that repository. An earlier layout let agents read the description; every comparison made that
  way was thrown away.

## Configurations and conditions

The configuration decides who finds. The condition decides how findings are verified.

| Configuration | Run as                               | Who reviews                                                      |
| ------------- | ------------------------------------ | ---------------------------------------------------------------- |
| panel         | `eyeful review`                      | The planner groups the change and Go picks the experts           |
| single        | `eyeful review generalist`           | One agent with every skill ([the roster](EXPERTS.md#the-roster)) |
| fixed         | `eyeful review correctness security` | Two named experts and no planner, for the [ablation](#ablations) |
| revmux        | `revmux.py`                          | An external reference, not the baseline: its prompts differ      |

| Condition | Verification                                                                     |
| --------- | -------------------------------------------------------------------------------- |
| execution | The verifier runs each reproduction; the judge reads the findings that have none |
| read-only | The [judge](VERIFICATION.md#the-judge) reads every finding                       |
| none      | Findings are reported as the experts wrote them                                  |

The three conditions verify one archive of findings, so verification is the only thing that
differs. Without a test command execution has nothing to run and equals read-only, so on Code Review
Bench and ReviewBench only read-only and none are compared.

## Procedure

Every subject is reviewed in each configuration three times. Execution runs first and the other
conditions reuse its findings. A review that fails is retried up to three times; one whose agents
opened a file outside the repository is recorded `leaked` and left out. Models, prompts, the budget
and this analysis plan are fixed before the main experiment starts.

## Measures and analysis

| Question | Measure                                                                                                                                                                                           | Analysis                                    |
| -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| RQ1      | Recall, counted per dataset as below; CWE categories; detections per token                                                                                                                        | McNemar's exact test between configurations |
| RQ2      | Actionable precision, A / (A + N), from the [labels](#labelling)                                                                                                                                  | Paired bootstrap over subjects              |
| RQ3      | Verifiable rate: reproductions passing all six [gates](VERIFICATION.md#the-verifier), the fix included. Verification loss: actionable findings execution does not confirm. Tokens, money, seconds | Bootstrap over subjects                     |

| Dataset           | A known defect counts as found when                                      | Scored by                    |
| ----------------- | ------------------------------------------------------------------------ | ---------------------------- |
| SWE-bench         | a finding is on its file within three lines of the lines the fix touches | `analyze.py`                 |
| Code Review Bench | the benchmark's judge prompt says a finding is the golden comment        | `match.py`                   |
| ReviewBench       | the benchmark's judge matches a comment to the golden finding            | the benchmark, or `score.py` |

Every estimate carries a 95% interval from 10,000 resamples of whole subjects. Checking eyeful's fix
against SWE-bench's own failing test is done by hand so far. TODO(eval): run the task's
`FAIL_TO_PASS` tests on eyeful's fix.

## Labelling {#labelling}

Every finding that is not on a known defect is labelled by the author, once per distinct finding.
It is **actionable** when all three hold:

1. the problem exists in the changed code as described;
2. the change introduces or exposes it;
3. it alters observable behaviour or violates a stated contract.

Anything else is **noise**. The judge applies the same three points.

The sheet shows findings in a fixed random order, without the tool, configuration or verification
outcome. A fifth are labelled again at least two weeks later, and Cohen's κ between the passes is
reported.

## Ablations {#ablations}

A part whose removal changes nothing measured is removed from eyeful.

| Removed        | Run as                    | Expected effect                                   |
| -------------- | ------------------------- | ------------------------------------------------- |
| The planner    | The `fixed` configuration | Recall on mixed changes drops, or nothing changes |
| Oracle grading | Computed from the archive | More findings block, and their precision drops    |

## Threats to validity

- One author labels every finding.
- The benchmarks judge with Claude Sonnet 5; the main experiment's model and judge are not chosen
  yet, and a cheaper judge is more lenient.
- SWE-bench's projects predate the model's cut-off, so the model may know the issue, and on a
  reverted subject the removed code is in the diff.
- ReviewBench's test pull requests are the ones the prompts were tuned on. Only the held-out part
  and the whole set give figures to cite.

## Tooling {#tooling}

The scripts in `eval/` use only Python's standard library, except `plot.py` and `score.py`, which
run with `uv run`. `mise run test` runs their tests.

| Script           | Does                                                                                                        |
| ---------------- | ----------------------------------------------------------------------------------------------------------- |
| `subjects.py`    | Builds subjects from SWE-bench rows or Code Review Bench's golden comments                                  |
| `reviewbench.py` | Builds ReviewBench subjects, draws the held-out part and writes eyeful's comments in the benchmark's format |
| `run.py`         | Reviews each subject with eyeful: each configuration finds, then the conditions verify                      |
| `revmux.py`      | Reviews each subject with revmux                                                                            |
| `confine.py`     | Starts pi in a macOS sandbox that reads the repository under review and nothing else of the user's          |
| `audit.py`       | Lists what a reviewer's agents opened outside the repository                                                |
| `match.py`       | Scores Code Review Bench reviews as the benchmark's offline pipeline does                                   |
| `score.py`       | Matches eyeful's comments to ReviewBench's golden findings with [Jev](ARCHITECTURE.md#jev)                  |
| `plot.py`        | Draws eyeful beside ReviewBench's leaderboard                                                               |
| `label.py`       | Writes the blind labelling sheet and computes κ                                                             |
| `analyze.py`     | Computes the measures from the reviews and the labels                                                       |
