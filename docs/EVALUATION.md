# Evaluation

This page defines how eyeful is evaluated. The plan is frozen in the
[proposal](https://github.com/Pleasurecruise/eyeful/blob/main/docs/reports/proposal.pdf); where this page and the proposal differ, the proposal
wins. The scripts in `eval/` are built; the runs are still to do ([Roadmap](ROADMAP.md)). Results
will be added once the main experiment has run.

## Research questions {#research-question}

Does planned multi-expert review with execution-based verification find more of the defects that
matter, with less noise and with evidence a maintainer can check, than simpler LLM reviewers, and
at what cost?

| Question                    | Asks                                                                                                                              |
| --------------------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| RQ1, detection              | Does a planned panel find more known defects, and more categories of problem, than a single agent with the same model and skills? |
| RQ2, noise                  | What proportion of reported findings is valid under each verification condition?                                                  |
| RQ3, verifiability and cost | How many findings can execution reproduce and fix, how many genuine defects does it fail to confirm, and at what cost?            |

No direction is assumed for RQ2. If read-only findings are already mostly valid, execution's value
lies in RQ3.

## Datasets

A subject is one change with known defects, from a public benchmark. Only SWE-bench has tests that
run, so it alone answers RQ3. No output is labelled by hand.

| Dataset                      | Subjects                                                                              | Known defects                            | Answers       |
| ---------------------------- | ------------------------------------------------------------------------------------- | ---------------------------------------- | ------------- |
| SWE-bench Verified, reverted | About 30 tasks from seven Python projects                                             | The task, resolved by its own tests      | RQ1, RQ2, RQ3 |
| Code Review Bench, offline   | 50 pull requests                                                                      | 173 comments human reviewers confirmed   | RQ1, RQ2      |
| ReviewBench                  | 50 of its 219 pull requests, sampled by language from the 194 not used in development | The benchmark's accepted golden findings | RQ1, RQ2      |

- A SWE-bench subject puts the reference fix in the base and takes it out in the change. It is
  built from the task's Docker image under a neutral name, with the issue text withheld, and its
  tests run in the task's container without a network. The seven projects are those `subjects.py`
  has a test runner for.
- ReviewBench's 25 test pull requests are the ones eyeful was developed on and are never sampled.
  The 50 are allotted to each language by its share of the other 194, and picked by hash, so the
  sample is the same every time.
- The datasets live under `.eval/`, which git ignores. A subject's description, which names the
  defect, is kept apart from its repository, and every review runs in a sandbox that reads only
  that repository. A run whose agents read outside it is excluded and counted.

## Configurations and conditions

The configuration decides who finds. The condition decides how findings are verified.

| Configuration | Run as                               | Who reviews                                                      |
| ------------- | ------------------------------------ | ---------------------------------------------------------------- |
| panel         | `eyeful review`                      | The planner groups the change and Go picks the experts           |
| single        | `eyeful review generalist`           | One agent with every skill ([the roster](EXPERTS.md#the-roster)) |
| fixed         | `eyeful review correctness security` | Two named experts and no planner, for the [ablation](#ablations) |

| Condition | Reported                                                             |
| --------- | -------------------------------------------------------------------- |
| none      | Every finding                                                        |
| read-only | The findings the [judge](VERIFICATION.md#the-judge) does not dispute |
| execution | The findings reproduced or, lacking a reproduction, not disputed     |

The three conditions verify one archive of findings, so verification is the only thing that
differs. The unit is the comment: findings [merged](VERIFICATION.md#one-problem) into one problem
count once. Without runnable tests execution equals read-only, so on Code Review Bench and
ReviewBench only read-only and none are compared.

## Procedure

The design is within-subject. Every subject is reviewed by the panel and by the single agent,
three times each. Execution runs first and the other conditions reuse its findings. The fixed
configuration runs once, on SWE-bench and Code Review Bench, and is compared with the panel's first
run. That is about 860 reviews. A review that fails is retried up to three times.

Every agent and judge uses DeepSeek V4.1 Flash, the benchmarks' official judges included, so the
results are internal comparisons, not leaderboard scores. Prompts, budgets and this plan are fixed
before the main experiment starts.

## Measures and analysis

Each dataset's own scorer decides whether a finding is valid:

| Dataset           | A known defect is found when                                                                                                             | Scored by        |
| ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------- | ---------------- |
| SWE-bench         | the official harness resolves the task with the fix agent's fix for a reported finding: its `FAIL_TO_PASS` and `PASS_TO_PASS` tests pass | `resolve.py`     |
| Code Review Bench | the benchmark's judge matches a finding to the golden comment, under its default Core profile                                            | `match.py`       |
| ReviewBench       | the benchmark's judge matches a finding to the golden finding; its classifier also credits valid unmatched ones                          | `reviewbench.py` |

| Question | Measure                                                                                                                                                                                                                      | Analysis                       |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------ |
| RQ1      | Recall: a known defect counts when two of a configuration's three runs find it. Categories found, in each benchmark's own labels                                                                                             | McNemar's exact test           |
| RQ2      | Precision: the share of reported findings that resolve the SWE-bench task or match a Code Review Bench golden comment; ReviewBench's augmented precision                                                                     | Paired bootstrap over subjects |
| RQ3      | Verifiable rate: executed findings whose reproduction passes all six [gates](VERIFICATION.md#the-verifier). Verification loss: findings that resolve a SWE-bench task but execution does not confirm. Tokens, money, seconds | Bootstrap over subjects        |

The panel is also compared with the union of the single agent's three runs, which costs about as
much as one panel run. Every estimate carries a 95% interval from 10,000 resamples of whole
subjects.

## Ablations {#ablations}

A part whose removal changes nothing measured is removed from eyeful.

| Removed        | Run as                                                              | Expected effect                                   |
| -------------- | ------------------------------------------------------------------- | ------------------------------------------------- |
| The planner    | The `fixed` configuration, against the panel's first run            | Recall on mixed changes drops, or nothing changes |
| Oracle grading | The archived reproductions rescored with every one allowed to block | More findings block, and their precision drops    |

## Threats to validity

- The LLM judges may be lenient.
- The golden sets are incomplete, so precision is a lower bound.
- One model is reviewer, judge and scorer.
- SWE-bench's projects predate the model's cut-off, so the model may know the issue, and on a
  reverted subject the removed code is in the diff.
- RQ3 rests on about 30 Python subjects.

## Tooling {#tooling}

The scripts in `eval/` use only Python's standard library. `resolve.py` runs SWE-bench's harness
with `uv`, and `match.py` and ReviewBench's judge run from the benchmarks' own checkouts.
`mise run test` runs the scripts' tests.

| Script           | Does                                                                                                            |
| ---------------- | --------------------------------------------------------------------------------------------------------------- |
| `subjects.py`    | Builds subjects from SWE-bench rows or Code Review Bench's golden comments                                      |
| `reviewbench.py` | Builds the ReviewBench sample, hands each review's reported comments to the benchmark's judge, reads its scores |
| `run.py`         | Reviews each subject with eyeful: each configuration finds, then the conditions verify                          |
| `confine.py`     | Starts pi in a macOS sandbox that reads the repository under review and nothing else of the user's              |
| `audit.py`       | Lists what a reviewer's agents opened outside the repository                                                    |
| `resolve.py`     | Runs each fix through SWE-bench's harness and scores the SWE-bench reviews                                      |
| `match.py`       | Runs Code Review Bench's judge on the reviews and scores them under the Core profile                            |
| `analyze.py`     | Computes the measures from the reviews and the scores, one dataset at a time                                    |
| `revmux.py`      | Reviews each subject with revmux, an optional extension scored on Code Review Bench                             |
