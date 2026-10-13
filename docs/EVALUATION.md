# Evaluation

This page defines the experiment of the final-year project and records its results. The tooling is
in `eval/` and pilots have run. The full datasets, the labelling and the main experiment are still
to do ([Roadmap](ROADMAP.md)).

**What the pilots support so far:** on public pull requests the panel finds more known problems
than a single agent, for about three times the tokens. Counted at the same number of comments it is
level with published review products, not ahead. Execution-based verification has no valid
measurement yet.

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

A subject is one change whose defects are known.

| Dataset                      | Size     | Role                                                                                                   |
| ---------------------------- | -------- | ------------------------------------------------------------------------------------------------------ |
| Injected defects             | about 60 | Primary. Real commits from repositories created after the models' cut-off, each with one defect put in |
| SWE-bench Verified, reverted | about 30 | Secondary. The base holds the reference fix and the change takes it out                                |
| Code Review Bench, offline   | 50       | External. Pull requests of five projects with 173 problems human reviewers confirmed. No test runs     |
| ReviewBench                  | 219      | External, and the set eyeful is developed against. 187 repositories, 19 languages. No test runs        |

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

Expert independence was tested this way and removed: experts run one after another found the same
and took twice as long ([Code Review Bench](#crb-pilot)).

## Tooling {#tooling}

The scripts in `eval/` use only Python's standard library, except `plot.py`, which runs with
`uv run`. `mise run test` runs their tests.

| Script           | Does                                                                                                                                                                                                                                   |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `inject.py`      | Screens repositories, picks real commits and puts one defect into each                                                                                                                                                                 |
| `subjects.py`    | Builds subjects from SWE-bench rows, injected specs or Code Review Bench's golden comments                                                                                                                                             |
| `run.py`         | Reviews each subject with eyeful: each configuration finds, then the conditions verify                                                                                                                                                 |
| `revmux.py`      | Reviews each subject with revmux                                                                                                                                                                                                       |
| `confine.py`     | Starts pi in a macOS sandbox that reads the repository under review and nothing else of the user's                                                                                                                                     |
| `audit.py`       | Lists what a reviewer's agents opened outside the repository; such a review is recorded `leaked`                                                                                                                                       |
| `match.py`       | Scores Code Review Bench reviews as the benchmark's offline pipeline does                                                                                                                                                              |
| `reviewbench.py` | Prepares ReviewBench subjects and writes eyeful's comments in the benchmark's input format. `table` writes one row per review; `standing` scores a run over every pull request judged, counting only the first comments of each report |
| `plot.py`        | Draws eyeful beside ReviewBench's leaderboard                                                                                                                                                                                          |
| `label.py`       | Writes the blind labelling sheet and computes κ                                                                                                                                                                                        |
| `analyze.py`     | Computes the measures from the reviews and the labels                                                                                                                                                                                  |

## Pilot {#pilot}

### SWE-bench, 18 subjects

**Invalid as a comparison.** It ran on 11 October 2026, before reviews were sandboxed, so reviewers
could read each subject's description. It shows only that the procedure runs.

Setup: 18 SWE-bench Verified subjects, one review per configuration, Claude Code on
`claude-sonnet-5-5`, revmux on its `claude-only` profile.

| Reviewer           | Recall            | Findings per review | Tokens (thousands) | Seconds     |
| ------------------ | ----------------- | ------------------- | ------------------ | ----------- |
| eyeful, panel      | 0.83 [0.67, 1.00] | 0.89 [0.67, 1.11]   | 89 [81, 98]        | 39 [33, 46] |
| eyeful, single     | 0.67 [0.44, 0.89] | 0.67 [0.44, 0.89]   | 58 [50, 67]        | 29 [23, 36] |
| revmux claude-only | 0.94 [0.83, 1.00] | 1.22 [1.00, 1.44]   | 326 [291, 368]     | 41 [36, 48] |

Neither difference is significant (McNemar p = 0.38 and 0.50). Under execution, 26 of 28 findings
came with a reproduction:

| Measure                                        | Value                                                  |
| ---------------------------------------------- | ------------------------------------------------------ |
| Verifiable rate: passed all six gates          | 0.62 [0.38, 0.82]                                      |
| Where the others stopped                       | `fails_as_claimed` 5, `pass_to_pass` 3, `introduced` 2 |
| Oracles of the confirmed findings              | `base` 11, `agent` 7, `implicit` 1                     |
| Blocking with the oracle rule, and without it  | 1, and 19                                              |
| Verification loss on the known defect          | 0.32 [0.11, 0.56]                                      |
| Tokens (thousands): none, read-only, execution | 80, 104, 89                                            |

- The oracle names are those the code had then: `existing` and `base` are now `derived`, `spec` is
  `specified`, `agent` is `human`.
- A reverted fix has no issue text, so the oracle rule turned 18 of 19 confirmed reproductions into
  questions.
- The three `pass_to_pass` stops are subjects whose suite came out empty, a limit of the builder.
- Asking once for a corrected reproduction, added afterwards, moved the verifiable rate from 0.62 to
  0.65 and the loss from 0.32 to 0.25 on the same findings.
- On 12 subjects, named experts without the planner and dependent experts each found 12 of 12,
  against 11 for the panel.

### Code Review Bench, seven pull requests {#crb-pilot}

Setup: 12 October 2026, seven pull requests (three Cal.com, two Sentry, two Grafana, 37 golden
comments), sandboxed, one run per configuration, pi on `opencode-go/deepseek-v4.1-flash` for every
agent and the match judge. The judge reads every finding.

| Reviewer                                         | Recall          | Findings a review | Tokens (thousands) | Seconds |
| ------------------------------------------------ | --------------- | ----------------- | ------------------ | ------- |
| eyeful panel, when the level still set the depth | 0.65 (20 of 31) | 13                | 472                | 486     |
| eyeful panel                                     | 0.86 (32 of 37) | 25                | 578                | 696     |
| eyeful, experts one after another                | 0.84 (21 of 25) | 20                | 796                | 1,598   |
| revmux `pi-only`                                 | 0.65 (24 of 37) | 6                 | 422                | 630     |
| **eyeful panel, held out**                       | 0.75 (15 of 20) | 16                | 517                | 645     |

- The first row covers five of the pull requests and the third covers four.
- **0.75 is the figure to quote.** The seven pull requests are the ones the prompts were adjusted
  on. The last row is seven Cal.com pull requests no run had touched, reviewed once with nothing
  changed.
- With one run each, a difference of one or two golden comments is noise.
- Leaving the depth to the experts is what moved recall from 0.65 to 0.86.
- The experts overlap: of 32 golden comments found, 21 were reported by more than one expert. Found
  by one expert only: tests 4, skeptic 3, design 2, correctness 1, security 1.
- Experts one after another found as much and took 2.2 times as long.
- One judge reading a group's findings together used 55% of the tokens of one judge per finding and
  gave 62 of 67 verdicts as before.
- Nothing is labelled, so there is no precision.

### ReviewBench, the test pull requests {#reviewbench-pilot}

Setup: 12 October 2026, the 25 test pull requests, one review per configuration, every agent on
`opencode-go/deepseek-v4.1-flash`, scored by the benchmark's judge on `deepseek-v4-flash` in place
of Claude Sonnet 5. Both model accounts ran out: ten pull requests have a score for both
configurations.

| Reviewer                  | Grounded recall | Grounded precision | Comments a pull request | Tokens (thousands) |
| ------------------------- | --------------- | ------------------ | ----------------------- | ------------------ |
| eyeful panel              | 0.41            | 0.82               | 16.0                    | 590                |
| eyeful panel, first five  | 0.26            |                    | 4.5                     |                    |
| eyeful panel, first three | 0.19            |                    | 2.9                     |                    |
| eyeful single agent       | 0.25            | 0.79               | 6.2                     | 190                |

- The panel found more golden findings than the single agent on seven of the ten and fewer on none.
- The judge is lenient: it called 93 of the panel's 104 unmatched comments real problems.
- The order of the comments is sound at the top: 55% of the first five match a golden finding, 25%
  of the sixth to tenth, 15% of the rest.
- Agreement predicts a match: 62% for a comment two or more experts reported, 19% for one.
- None of the 11 comments the judge disputed matched a golden finding.
- Nitpicks are 76 of the 160 comments, and 16% of them match.
- The gain over the single agent is in medium and low findings (0.55 against 0.38, 0.27 against
  0.09). On high-severity findings the panel found 5 of 11 and the single agent 6.

### ReviewBench, the test set at `dbf9e50` {#reviewbench-dbf9e50}

Setup: 12 and 13 October 2026, the same 25 pull requests, the binary built from commit `dbf9e50`,
the same model. The panel reviewed all 25, two of them `partial` because the two largest changes
used up the budget. The single agent reviewed 20 before the provider's limit. One row per review is
in [`reviewbench-test.csv`](/eval/reviewbench-test.csv); the `before` rows are the earlier pilot.

**Recall.** The judge, on `deepseek-v4.1-flash`, scored 17 of the panel's reviews and none of the
single agent's. The eight it did not reach include both `partial` reviews.

| Panel, 17 pull requests | Grounded recall   | Grounded precision | Comments a pull request |
| ----------------------- | ----------------- | ------------------ | ----------------------- |
| every comment           | 0.393 (92 of 234) | 0.788              | 18.6                    |
| first five              | 0.231             | 0.882              | 4.8                     |

On the 15 pull requests scored in both runs the panel found 76 of 196 golden findings before and 84
at `dbf9e50`, with 15.7 and 19.5 comments. With one review each this is not told apart from noise.

**Cost.** On the pull requests reviewed in both runs:

| Per review        | Pull requests | Comments | Tokens (thousands) | Seconds |
| ----------------- | ------------- | -------- | ------------------ | ------- |
| Panel, before     | 21            | 16.4     | 592                | 698     |
| Panel, `dbf9e50`  | 21            | 20.5     | 647                | 581     |
| Single, before    | 19            | 6.5      | 186                | 409     |
| Single, `dbf9e50` | 19            | 7.8      | 185                | 404     |

On the 20 pull requests both configurations finished, the panel reads 3.2 times what the single
agent reads:

| Tokens a review, thousands | Experts | Judge | Planner | In all |
| -------------------------- | ------- | ----- | ------- | ------ |
| Panel                      | 435     | 130   | 24      | 589    |
| Single agent               | 129     | 54    | 0       | 183    |

A change a hundred times larger costs the panel about ten times as much.

#### Beside the leaderboard {#reviewbench-standing}

ReviewBench publishes a [leaderboard](https://review-bench.ai) of seven review products. The figure
shows where eyeful's panel stands. **It is not a ranking:**

- The leaderboard is scored on all 219 pull requests, eyeful on 23 of the 25 test pull requests.
  The benchmark says a score on that set shows an adapter works, not how good a reviewer is.
- The 25 are the pull requests eyeful is developed on.
- The leaderboard is judged by Claude Sonnet 5, eyeful by `deepseek-v4.1-flash`, which is lenient.
- Each pull request was reviewed once.

![eyeful beside the ReviewBench leaderboard](/eval/reviewbench-standing.svg)

The figure uses the `before` run, which has the most scores: 23 pull requests, four of them never
reviewed because the model accounts ran out. Those four count as finding nothing. Leaving them out
would give 0.377 in place of 0.315.

| eyeful panel  | Comments a pull request | Grounded recall | Grounded precision |
| ------------- | ----------------------- | --------------- | ------------------ |
| every comment | 12.9                    | 0.315           | 0.796              |
| first five    | 3.8                     | 0.181           | 0.807              |

At five comments, which is about what the products hand in, eyeful is level with Codex and below
Copilot, Devin and Qodo, and its precision is below their 0.84 to 0.88. The leaderboard's rows are
in [`reviewbench-leaderboard.csv`](/eval/reviewbench-leaderboard.csv) as read on 13 October 2026,
and eyeful's in [`reviewbench-standing.csv`](/eval/reviewbench-standing.csv).
