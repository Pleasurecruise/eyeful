# Evaluation

This page is part of the final-year project. It defines the experiment that answers the research
questions, and its results are also the figures published about eyeful. The experiment runs in v1.0,
on the research core, before any other client is built ([Roadmap](ROADMAP.md)). The tooling is in
`eval/` and a pilot has run on SWE-bench subjects ([Pilot](#pilot)); the full datasets, the
labelling and the main experiment are still to do.

## Research questions {#research-question}

The aim is to determine whether planned multi-expert review with execution-based verification finds
more of the defects that matter, with less noise and with evidence a maintainer can check, than
simpler LLM reviewers, and at what cost. The questions cover correctness and security findings, the
ones that can be reproduced:

| Question           | Asks                                                                                                                                          |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- |
| RQ1, detection     | Does a planned panel of experts find more known defects, and more categories of problem, than a single agent using the same model and skills? |
| RQ2, noise         | What proportion of reported findings is actionable under each verification condition?                                                         |
| RQ3, verifiability | How many findings can execution confirm, how many genuine defects does it fail to confirm, and at what cost in tokens, money and time?        |

No direction is assumed for RQ2. If read-only findings are already mostly actionable, the value of
execution lies in RQ3: a failing test and a tested fix in place of an argument.

## Configurations and conditions

Two things vary. The configuration decides who finds, and the condition decides how findings are
verified:

| Configuration | Run as                     | Who reviews                                                                        |
| ------------- | -------------------------- | ---------------------------------------------------------------------------------- |
| panel         | `eyeful review`            | The planner groups the change and Go picks the experts                             |
| single        | `eyeful review generalist` | One agent with every skill the experts share ([the roster](EXPERTS.md#the-roster)) |

| Condition | Verification                                                                                                                       |
| --------- | ---------------------------------------------------------------------------------------------------------------------------------- |
| execution | The verifier runs each reproduction through its [gates](VERIFICATION.md#the-verifier); the judge reads the findings that have none |
| read-only | A [judge agent](VERIFICATION.md#the-judge) reads the code and each finding and gives a verdict                                     |
| none      | Findings are reported as the experts wrote them                                                                                    |

A review picks its condition with `Request.Verification`: `execution`, `read_only` or `none`. The
run archive keeps the verifier's and the summarizer's output under the condition's name and
everything before them once, so running the three conditions over one archive reuses the same plan
and the same findings, and verification is the only thing that differs. Experts find under
`execution`, because only then are they told the reproduction command and hand in tests.

revmux, whose verify stage is read-only, also runs on the same subjects as an external reference.
It is not the baseline, because its prompts and splitting differ from eyeful's and those differences
would mix into the result.

## Datasets

A subject is one change, shaped like a pull request, whose defects are known.

| Dataset                      | Size     | Role                                                                                                                                                                                                                                                                                                                                                    |
| ---------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Injected defects             | about 60 | Primary. Real commits from repositories created after the models' training cut-off, each with one defect of a fixed type put in: a boundary error, a missing check, a wrong condition, a resource leak or an injection flaw                                                                                                                             |
| SWE-bench Verified, reverted | about 30 | Secondary. The base holds the reference fix and the change takes it out. Reported separately, because the models may have seen these repositories                                                                                                                                                                                                       |
| Code Review Bench, offline   | 50       | External. Real pull requests of Sentry, Grafana, Cal.com, Discourse and Keycloak, each with the problems human reviewers confirmed in it, 173 in all. A pull request holds several, so it measures how much of a change a review covers, which one defect per subject cannot. No test runs in these repositories, so every finding is judged by reading |

A SWE-bench subject is built from the task's own Docker image. The commit message of the change
names only the file, the issue text is withheld, and the image is tagged under a neutral name, so
nothing in the repository says which task it is. The project commands run each test in a fresh
container without a network, over a copy of the checkout; the suite is the task's pass-to-pass tests
that pass there. The builder covers the projects whose tests run with pytest or Django's runner.

A reviewer that can read more than the change is reviewing with the answer beside it. A subject's
description, which holds the lines of the defect, is kept apart from its repository, and every
review runs with its agents in a sandbox that reads only that repository and starts no program that
downloads. An earlier layout kept the description beside the repository, and agents read it, other
runs' results and, where they had a shell, the project's later releases; every comparison made that
way was thrown away.

## Procedure

The design is within-subject: every subject is reviewed under every configuration and condition,
which removes differences in difficulty between subjects. Each configuration reviews a subject three
times, because model output varies between runs, and the archived findings of every run then pass
through the three conditions. Models, prompts, the budget of a review and this analysis plan are
fixed before the main experiment starts.

## Labelling {#labelling}

Whether the known defect was found needs no labelling: a finding counts when it is on the file of
the defect, within three lines of the lines the defect changed. Everything else the reviewers report
is labelled by the author, once per distinct finding, where findings on the same file within two
lines of each other are one finding.

A finding is **actionable** only when all three hold:

1. the problem exists in the changed code as the finding describes it;
2. the change introduces or exposes it, and did not merely leave it as it was;
3. it alters behaviour someone can observe, or violates a stated contract: an issue, documentation,
   an API schema or an existing test.

Anything else is **noise**: style, naming and wording; a concern that names no input or sequence
that goes wrong; a problem that was already there; a duplicate of the contract the code already
keeps. The judge applies the same three points, so the conditions differ in evidence and not in what
they count as a defect.

The sheet shows the subject, the place and the text of each finding, in a fixed random order,
without the tool, the configuration or the verification outcome. A fifth of the findings are
labelled again at least two weeks later, and Cohen's κ between the passes is reported as intra-rater
agreement. If the supervisor agrees, a further sample is labelled independently. Labelling by one
author is a threat to validity and the thesis says so.

## Measures and analysis

| Question | Measure                                                                                                                                                                                        | Analysis                                    |
| -------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| RQ1      | Recall: the share of subjects whose known defect is reported; CWE categories found; detections per million tokens                                                                              | McNemar's exact test between configurations |
| RQ2      | Actionable precision, A / (A + N), of what each condition reports                                                                                                                              | Paired bootstrap over subjects              |
| RQ3      | Verifiable rate: the share of reproductions that pass all six gates. Verification loss: the share of actionable findings execution does not confirm. Tokens, money and seconds for each review | Bootstrap over subjects                     |

Every estimate carries a 95% interval from 10,000 resamples of whole subjects. Besides single-author
labelling, the threats to validity are how realistic injected defects are and which languages the
subjects cover.

## Ablations {#ablations}

Each part of the workflow has to show that it helps. It is removed, the experiment is rerun, and the
change is measured.

| Removed             | Run as                               | Expected effect                                       |
| ------------------- | ------------------------------------ | ----------------------------------------------------- |
| The planner         | `eyeful review correctness security` | Recall on mixed changes drops, or nothing changes     |
| Oracle grading      | Computed from the archive            | More findings block, and precision of those drops     |
| Expert independence | `eyeful review --dependent`          | Fewer distinct findings, and corroboration means less |

`--dependent` is a hidden flag that exists for these runs. A part whose removal changes
nothing measured is removed from eyeful.

## Tooling {#tooling}

The scripts in `eval/` use only Python's standard library, and `mise run test` runs their tests.
Each takes `--help`.

| Script        | Does                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `inject.py`   | Screens repositories for a pytest suite that passes in a plain Python image, picks real commits and puts one defect into each                                                                                                                                                                                                                                                                                                                                  |
| `subjects.py` | Builds subjects from SWE-bench Verified rows, from the injected set's specs or from Code Review Bench's golden comment files. A subject's description, which names the defect, and the repository the reviewers work in go to separate places, the repository under a name that says nothing about it                                                                                                                                                          |
| `run.py`      | Reviews each subject with eyeful: every configuration finds once per repetition, then the conditions verify. `--pi` fixes the model and thinking level of every pi call and records them beside the reviews                                                                                                                                                                                                                                                    |
| `revmux.py`   | Reviews each subject with revmux. The `pi-only` profile needs a revmux build with a pi executor, which upstream does not have                                                                                                                                                                                                                                                                                                                                  |
| `confine.py`  | Starts pi in a macOS sandbox that reads the repository under review, pi's own files and nothing else of the user's; cannot change the repository; cannot read pi's saved sessions or any installed Python package; and cannot start `curl`, `pip`, `python` or git's HTTP helper. It gives the agents a private temporary directory and a git that reads no user configuration. `run.py` and `revmux.py` run every review through it, in a temporary directory |
| `audit.py`    | Lists what a reviewer's agents opened outside the repository, among the places the sandbox leaves readable that are neither the system nor a toolchain; a review with any is recorded as `leaked`                                                                                                                                                                                                                                                              |
| `match.py`    | Scores reviews of Code Review Bench subjects as the benchmark's offline pipeline does: a model judges every reported finding against every golden comment of the pull request with the benchmark's own prompt; recall is the golden comments matched; precision is the same count over the findings, and also over the places they point to, where findings on the same lines count once                                                                       |
| `label.py`    | Writes the blind labelling sheet and the second-pass sample, and computes κ                                                                                                                                                                                                                                                                                                                                                                                    |
| `analyze.py`  | Computes the measures above from the reviews and the labels                                                                                                                                                                                                                                                                                                                                                                                                    |

Every number comes from the run archive: the run directory of each review holds the snapshot, the
diff, every stage's checkpoint with the tokens and money each agent call reported, and the result
with each verification and what its runs showed, from which `run.py` reads the gate it stopped at.

## Pilot {#pilot}

The pilot checked that the whole procedure runs; it is not the experiment, and it ran before
reviews were sandboxed, so its reviewers could read each subject's description and its figures say
nothing about either tool's ability. It ran on 11 October
2026 over 18 SWE-bench Verified subjects (6 from Requests, 5 from pytest, 3 from Django, 3 from
Pylint and 1 from Flask), taken in instance order from the tasks whose reference fix touches one
file and is under 1,500 bytes. Each configuration reviewed each subject once, not three times. The
agent was Claude Code with every model tier set to `claude-sonnet-5-5`, and revmux ran its
`claude-only` profile on the same model. Nothing is labelled yet, so there is no precision.

| Reviewer           | Recall            | Findings per review | Tokens (thousands) | Seconds     |
| ------------------ | ----------------- | ------------------- | ------------------ | ----------- |
| eyeful, panel      | 0.83 [0.67, 1.00] | 0.89 [0.67, 1.11]   | 89 [81, 98]        | 39 [33, 46] |
| eyeful, single     | 0.67 [0.44, 0.89] | 0.67 [0.44, 0.89]   | 58 [50, 67]        | 29 [23, 36] |
| revmux claude-only | 0.94 [0.83, 1.00] | 1.22 [1.00, 1.44]   | 326 [291, 368]     | 41 [36, 48] |

eyeful's figures are for the execution condition. The panel found the defect on four subjects where
the single agent did not, and the single agent on one where the panel did not (McNemar p = 0.38);
revmux found it on two where the panel did not (p = 0.50). Neither difference is significant at this
size. revmux reads about 3.7 times as many tokens as the panel for a review that takes as long.
Every one of eyeful's 28 findings was on the known defect; 3 of revmux's 22 were elsewhere and wait
for labels, as do the 20 distinct findings on the sheet.

Under execution, 26 of the 28 findings came with a reproduction:

| Measure                                        | Value                                                  |
| ---------------------------------------------- | ------------------------------------------------------ |
| Verifiable rate: passed all six gates          | 0.62 [0.38, 0.82]                                      |
| Where the others stopped                       | `fails_as_claimed` 5, `pass_to_pass` 3, `introduced` 2 |
| Oracles of the confirmed findings              | `base` 11, `agent` 7, `implicit` 1                     |
| Blocking with the oracle rule, and without it  | 1, and 19                                              |
| Verification loss on the known defect          | 0.32 [0.11, 0.56]                                      |
| Tokens (thousands): none, read-only, execution | 80, 104, 89                                            |

A reverted fix comes with no issue text, so almost no finding has an outside oracle, and the oracle
rule turns 18 of 19 confirmed reproductions into questions. The injected set, whose changes carry
their own commit messages, is where `spec` oracles can appear. Execution cost less than the judge
here, because the judge reads the code again for every finding. The three `pass_to_pass` stops are subjects
whose suite came out empty, which is a limit of the builder and not of the findings.

The step that asks once for a corrected reproduction was added after these runs and tried on the
same archived findings: of the five reproductions that failed for another reason than the one
claimed, three then ran as claimed, the verifiable rate moved from 0.62 to 0.65, and the loss on the
known defect from 0.32 to 0.25.

The two ablations that need a new run were tried once on 12 of the subjects: named experts
without the planner and dependent experts each found 12 of 12 defects, against 11
of 12 for the panel, with about one finding and 80 to 89 thousand tokens a review. Small changes
with one defect do not separate them.
