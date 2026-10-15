# Evaluation

This page defines the experiment of the final-year project and records its results. The tooling is
in `eval/` and pilots have run. The full datasets, the labelling and the main experiment are still
to do ([Roadmap](ROADMAP.md)).

So far, on one Code Review Bench pull request the panel found one of two known problems and missed
the critical one, and the single agent did not finish. On one SWE-bench issue it found, reproduced
and fixed the defect, but reported it only as a question. Nothing is labelled yet, so precision is
unknown.

## Research questions {#research-question}

Does planned multi-expert review with execution-based verification find more of the defects that
matter, with less noise and with evidence a maintainer can check, than simpler LLM reviewers, and
at what cost?

| Question           | Asks                                                                                                      |
| ------------------ | --------------------------------------------------------------------------------------------------------- |
| RQ1, detection     | Does a planned panel find more known defects than a single agent with the same model and skills?          |
| RQ2, noise         | What proportion of reported findings is actionable under each verification condition?                     |
| RQ3, verifiability | How many findings can execution confirm, how many real defects does it fail to confirm, and at what cost? |

## Configurations and conditions

The configuration decides who finds. The condition decides how findings are verified.

| Configuration | Run as                     | Who reviews                                                      |
| ------------- | -------------------------- | ---------------------------------------------------------------- |
| panel         | `eyeful review`            | The planner groups the change and Go picks the experts           |
| single        | `eyeful review generalist` | One agent with every skill ([the roster](EXPERTS.md#the-roster)) |

| Condition | Verification                                                                     |
| --------- | -------------------------------------------------------------------------------- |
| execution | The verifier runs each reproduction; the judge reads the findings that have none |
| read-only | The [judge](VERIFICATION.md#the-judge) reads every finding                       |
| none      | Findings are reported as the experts wrote them                                  |

The three conditions run over one archive of findings, so verification is the only thing that
differs. revmux runs on the same subjects as an external reference, not as the baseline: its
prompts and splitting differ.

## Datasets

A subject is one change whose defects are known. Every subject comes from a public benchmark.

| Dataset                      | Size     | Role                                                                                                    |
| ---------------------------- | -------- | ------------------------------------------------------------------------------------------------------- |
| SWE-bench Verified, reverted | about 30 | Primary, and the only set whose tests run. The base holds the reference fix and the change takes it out |
| Code Review Bench, offline   | 50       | External. Pull requests of five projects with 173 problems human reviewers confirmed. No test runs      |
| ReviewBench                  | 219      | External, and the set eyeful is developed against. 187 repositories, 19 languages. No test runs         |

ReviewBench's 25 test pull requests are used while the harness changes, a held-out part is kept for
checking a change once, and the whole set is for the final run.

- The datasets live under `.eval/`, which git ignores.
- A SWE-bench subject is built from the task's Docker image under a neutral name, with the issue
  text withheld. Tests run in a fresh container without a network.
- A subject's description, which names the defect, is kept apart from its repository, and every
  review runs in a sandbox that reads only that repository. An earlier layout let agents read the
  description; every comparison made that way was thrown away.

## Procedure

Every subject is reviewed under every configuration and condition, three times each. Models,
prompts, the budget and this analysis plan are fixed before the main experiment starts.

## Labelling {#labelling}

The known defect needs no labelling: a finding counts when it is on the defect's file within three
lines of the changed lines. Everything else is labelled by the author, once per distinct finding.

A finding is **actionable** when all three hold:

1. the problem exists in the changed code as described;
2. the change introduces or exposes it;
3. it alters observable behaviour or violates a stated contract.

Anything else is **noise**. The judge applies the same three points.

The sheet shows findings in a fixed random order, without the tool, configuration or verification
outcome. A fifth are labelled again at least two weeks later, and Cohen's κ between the passes is
reported. Labelling by one author is a threat to validity.

## Measures and analysis

| Question | Measure                                                                                                                                         | Analysis                                    |
| -------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| RQ1      | Recall: the share of subjects whose known defect is reported; CWE categories; detections per token                                              | McNemar's exact test between configurations |
| RQ2      | Actionable precision, A / (A + N)                                                                                                               | Paired bootstrap over subjects              |
| RQ3      | Verifiable rate: reproductions passing all six gates. Verification loss: actionable findings execution does not confirm. Tokens, money, seconds | Bootstrap over subjects                     |

Every estimate carries a 95% interval from 10,000 resamples of whole subjects.

## Ablations {#ablations}

A part whose removal changes nothing measured is removed from eyeful.

| Removed        | Run as                               | Expected effect                                   |
| -------------- | ------------------------------------ | ------------------------------------------------- |
| The planner    | `eyeful review correctness security` | Recall on mixed changes drops, or nothing changes |
| Oracle grading | Computed from the archive            | More findings block, and their precision drops    |

Expert independence was tested this way and removed: on four Code Review Bench pull requests,
experts run one after another found 21 of 25 golden comments, as many as the panel, and took 2.2
times as long.

## Tooling {#tooling}

The scripts in `eval/` use only Python's standard library, except `plot.py` and `score.py`, which
run with `uv run`. `mise run test` runs their tests.

| Script           | Does                                                                                                                                                                                                                                   |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `subjects.py`    | Builds subjects from SWE-bench rows or Code Review Bench's golden comments                                                                                                                                                             |
| `run.py`         | Reviews each subject with eyeful: each configuration finds, then the conditions verify                                                                                                                                                 |
| `revmux.py`      | Reviews each subject with revmux                                                                                                                                                                                                       |
| `confine.py`     | Starts pi in a macOS sandbox that reads the repository under review and nothing else of the user's                                                                                                                                     |
| `audit.py`       | Lists what a reviewer's agents opened outside the repository; such a review is recorded `leaked`                                                                                                                                       |
| `match.py`       | Scores Code Review Bench reviews as the benchmark's offline pipeline does                                                                                                                                                              |
| `reviewbench.py` | Prepares ReviewBench subjects and writes eyeful's comments in the benchmark's input format. `table` writes one row per review; `standing` scores a run over every pull request judged, counting only the first comments of each report |
| `score.py`       | Matches eyeful's comments to ReviewBench's golden findings with [Jev](ARCHITECTURE.md#jev) and writes the grounded scores in the shape `table` and `standing` read                                                                     |
| `plot.py`        | Draws eyeful beside ReviewBench's leaderboard                                                                                                                                                                                          |
| `label.py`       | Writes the blind labelling sheet and computes κ                                                                                                                                                                                        |
| `analyze.py`     | Computes the measures from the reviews and the labels                                                                                                                                                                                  |

## Pilot {#pilot}

One result per dataset, from the best harness so far. Each ran once on a cheap model, so these show
what works; the figures to cite come from the main experiment. pi was given `--thinking off`, and
the model still reasoned in most turns.

### Code Review Bench, one pull request {#crb-pilot}

`keycloak/keycloak#32918`, two golden comments, untouched by any earlier run. Every agent on
`volcengine/deepseek-v4.1-flash`, matched by `match.py` with `deepseek/deepseek-flash` as the judge.

| Reviewer     | Recall        | Findings | Comments | Tokens (thousands) | Seconds |
| ------------ | ------------- | -------- | -------- | ------------------ | ------- |
| Panel        | 0.50 (1 of 2) | 22       | 15       | 1,206              | 1,239   |
| Single agent | —             | —        | —        | —                  | > 1,200 |

One pull request with two golden comments gives no estimate. The panel found the medium one (a test
cleans up the wrong alias) and missed the critical one (`getForLogin` looks providers up through
`session.identityProviders()`). The single agent hit its 20-minute limit three times and returned
nothing.

### One SWE-bench subject, end to end {#swebench-case}

`pytest-dev__pytest-10356`, rated 1 to 4 hours by SWE-bench. The change makes
class marks come from `getattr` again instead of every class in the MRO, so a class with two marked
bases loses one base's marks without any error. Sandboxed, every agent on `volcengine/deepseek-v4.1-flash`. SWE-bench's test for the issue, `test_mark_mro`, was kept
out of the repository; it passes on the base and fails on the change.

| Step      | Result                                                                          |
| --------- | ------------------------------------------------------------------------------- |
| Find      | Both changed regions, each reported as an error by correctness and skeptic      |
| Reproduce | 2 tests; each failed 5 of 5 times on the change and passed on the base          |
| Fix       | Both `fix_verified`; the 65 suite tests still pass                              |
| Regress   | With eyeful's fix, `test_mark_mro` passes; `test_mark.py`: 89 passed, 1 xfailed |
| Report    | 2 questions, 1 nitpick; nothing blocks                                          |

| Condition | Tokens (thousands) | Seconds |
| --------- | ------------------ | ------- |
| execution | 565                | 779     |
| read-only | 419                | 607     |
| none      | 364                | 475     |

Nothing blocks because both oracles are `derived`: no existing test or quoted text says marks
merge. eyeful's fix is the upstream fix byte for byte, but on a reverted subject the removed code is
in the diff. pytest predates the model's cut-off, so the model may know the issue.

### Jev, on the archived reviews {#jev-pilot}

`jev-1.13.0`, asked over the archived output of the 25 panel reviews of the ReviewBench test set
(748 findings). No review was run again.

| Judgement                                                 | Compared with                | Result                                              | Decided by                                  |
| --------------------------------------------------------- | ---------------------------- | --------------------------------------------------- | ------------------------------------------- |
| Two findings within two lines are one problem, 318 pairs  | 35 pairs read by hand        | 204 at 0.9 or more, 80 under 0.2; the sample agrees | Jev                                         |
| A comment is about a golden finding, 2,498 pairs          | The benchmark's judge        | 98.3% of pairs the same; 115 golden found, not 117  | Jev, while the harness changes              |
| The judge's three points from a file's diff, 314 findings | The judge's verdicts         | 4 of its 26 rejections; AUC 0.69                    | The judge: the callers are not in a diff    |
| Severity from a finding's text, 376 comments              | Whether a golden finding hit | AUC 0.73, the expert's 0.71                         | The expert                                  |
| Area of a comment, 376 comments                           | The areas of its experts     | 299 inside them                                     | The experts' areas                          |
| Category of a file, 245 files                             | The planner's group          | 72% the same category, 80% the same experts         | The planner: it still groups and summarises |

Position alone merged 97 pairs that are different problems; with Jev the 748 findings become 589
comments, against 569 by position. `score.py` follows the benchmark's judge (r = 0.99 over 19 pull
requests) but does not classify comments that match nothing, so a figure to cite still comes from
the benchmark's judge. The 0.5 threshold was fixed before looking.
