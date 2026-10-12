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
run archive keeps the verifier's output under the condition's name and
everything before them once, so running the three conditions over one archive reuses the same plan
and the same findings, and verification is the only thing that differs. Experts find under
`execution`, because only then are they told the reproduction command and hand in tests.

revmux, whose verify stage is read-only, also runs on the same subjects as an external reference.
It is not the baseline, because its prompts and splitting differ from eyeful's and those differences
would mix into the result.

## Datasets

A subject is one change, shaped like a pull request, whose defects are known.

| Dataset                      | Size     | Role                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ---------------------------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| Injected defects             | about 60 | Primary. Real commits from repositories created after the models' training cut-off, each with one defect of a fixed type put in: a boundary error, a missing check, a wrong condition, a resource leak or an injection flaw                                                                                                                                                                                                    |
| SWE-bench Verified, reverted | about 30 | Secondary. The base holds the reference fix and the change takes it out. Reported separately, because the models may have seen these repositories                                                                                                                                                                                                                                                                              |
| Code Review Bench, offline   | 50       | External. Real pull requests of Sentry, Grafana, Cal.com, Discourse and Keycloak, each with the problems human reviewers confirmed in it, 173 in all. A pull request holds several, so it measures how much of a change a review covers, which one defect per subject cannot. No test runs in these repositories, so every finding is judged by reading                                                                        |
| ReviewBench                  | 219      | External, and the set eyeful is developed against. Public pull requests of 187 repositories in 19 languages, each with the findings GitHub's benchmark accepts as true or false, about twelve true ones a pull request. Its 25 test pull requests are used while the harness changes, a part of the rest is kept for checking a change once, and the whole set is for the final run. No test runs in these repositories either |

The datasets live under `.eval/` in the repository, which git ignores: one directory for each
dataset, holding the benchmark's own files, the subjects' descriptions, the repositories under
names that say nothing about them, and the runs and scores. Nothing of an evaluation is kept in the
home directory or in a temporary directory.

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

| Removed        | Run as                               | Expected effect                                   |
| -------------- | ------------------------------------ | ------------------------------------------------- |
| The planner    | `eyeful review correctness security` | Recall on mixed changes drops, or nothing changes |
| Oracle grading | Computed from the archive            | More findings block, and precision of those drops |

A part whose removal changes nothing measured is removed from eyeful. Expert independence was
tested this way and the flag for it is gone: with the experts run one after another, each shown the
findings before it, recall was the same and the review took twice as long
([Code Review Bench](#crb-pilot)).

## Tooling {#tooling}

The scripts in `eval/` use only Python's standard library, and `mise run test` runs their tests.
Each takes `--help`.

| Script           | Does                                                                                                                                                                                                                                                                                                                                                                                                                                                           |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `inject.py`      | Screens repositories for a pytest suite that passes in a plain Python image, picks real commits and puts one defect into each                                                                                                                                                                                                                                                                                                                                  |
| `subjects.py`    | Builds subjects from SWE-bench Verified rows, from the injected set's specs or from Code Review Bench's golden comment files. A subject's description, which names the defect, and the repository the reviewers work in go to separate places, the repository under a name that says nothing about it                                                                                                                                                          |
| `run.py`         | Reviews each subject with eyeful: every configuration finds once per repetition, then the conditions verify. `--pi` fixes the model and thinking level of every pi call and records them beside the reviews                                                                                                                                                                                                                                                    |
| `revmux.py`      | Reviews each subject with revmux. The `pi-only` profile needs a revmux build with a pi executor, which upstream does not have                                                                                                                                                                                                                                                                                                                                  |
| `confine.py`     | Starts pi in a macOS sandbox that reads the repository under review, pi's own files and nothing else of the user's; cannot change the repository; cannot read pi's saved sessions or any installed Python package; and cannot start `curl`, `pip`, `python` or git's HTTP helper. It gives the agents a private temporary directory and a git that reads no user configuration. `run.py` and `revmux.py` run every review through it, in a temporary directory |
| `audit.py`       | Lists what a reviewer's agents opened outside the repository, among the places the sandbox leaves readable that are neither the system nor a toolchain; a review with any is recorded as `leaked`                                                                                                                                                                                                                                                              |
| `match.py`       | Scores reviews of Code Review Bench subjects as the benchmark's offline pipeline does: a model judges every reported finding against every golden comment of the pull request with the benchmark's own prompt; recall is the golden comments matched; precision is the same count over the findings, and also over the places they point to, where findings on the same lines count once                                                                       |
| `reviewbench.py` | Prepares ReviewBench pull requests as subjects, fetched from the benchmark's mirror, and writes eyeful's comments on them in the benchmark's judging input format, the comments shown on their own by default and the folded nitpicks too with `--folded`. The benchmark's own judge then scores them                                                                                                                                                          |
| `label.py`       | Writes the blind labelling sheet and the second-pass sample, and computes κ                                                                                                                                                                                                                                                                                                                                                                                    |
| `analyze.py`     | Computes the measures above from the reviews and the labels                                                                                                                                                                                                                                                                                                                                                                                                    |

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

Under execution, 26 of the 28 findings came with a reproduction. The oracles, severities and
evidence kinds on this page are recorded under the names the code had when each run was made;
`existing` and `base` are now `derived`, `spec` is `specified` and `agent` is `human`
([the oracle problem](VERIFICATION.md#oracle)):

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

### Code Review Bench, seven pull requests {#crb-pilot}

On 12 October 2026 seven of the benchmark's pull requests (three of Cal.com, two of Sentry, two of
Grafana, with 37 golden comments between them) were reviewed in the sandbox, once per
configuration, with pi on `opencode-go/deepseek-v4.1-flash` for every agent and for the match
judge. No test runs in these repositories, so the judge reads every finding. With one run each, a
difference of one or two golden comments is no more than two runs of one configuration differ by.

| Reviewer                                              | Recall          | Findings a review | Tokens (thousands) | Seconds |
| ----------------------------------------------------- | --------------- | ----------------- | ------------------ | ------- |
| eyeful panel, when the level still set the depth      | 0.65 (20 of 31) | 13                | 472                | 486     |
| eyeful panel                                          | 0.86 (32 of 37) | 25                | 578                | 696     |
| eyeful, experts one after another                     | 0.84 (21 of 25) | 20                | 796                | 1,598   |
| revmux `pi-only`                                      | 0.65 (24 of 37) | 6                 | 422                | 630     |
| eyeful panel, seven pull requests it was not tuned on | 0.75 (15 of 20) | 16                | 517                | 645     |

The first row is five of the pull requests and the third is four, on which the panel also found 21
of 25; the third row was run before the judge read a group at a time
([the judge](VERIFICATION.md#the-judge)), which is most of its extra tokens. Leaving the depth to
the experts is what moved recall. The panel reports its 25 findings in 15 places and they match 4.6 golden
comments a review, against 3.4 for revmux's 6.3 findings; the places that match none are not
labelled, so this says nothing yet about precision.

The experts overlap heavily: of the 32 golden comments the panel found, 21 were reported by more
than one expert, and the ones only a single expert reported were 4 for tests, 3 for the skeptic, 2
for design and 1 each for correctness and security. Running the experts one after another found as
much with a quarter fewer findings and took 2.2 times as long. A middle form, in which only the
skeptic started after the others and was told their findings, found 33 of 37 for the same tokens
and a third more time, and was not kept: one golden comment is inside the noise.

The seven pull requests above are also the ones the expert prompts were adjusted on, so 0.86 is a
figure for a development set. The last row is the check against that: the seven Cal.com pull
requests of the benchmark that no run had touched, all TypeScript, reviewed once with nothing
changed in between. Recall there is 0.75, eleven points lower, against 0.86 (18 of 21) on the three
Cal.com pull requests of the development set. With 20 golden comments the two cannot be told apart
for certain, but the direction is the one over-fitting gives, and 0.75 is the figure to quote. Of
the five comments missed, two are about wording or portability of a script, and three are
functional: a guard that leaves out organisations without teams, a nullable column without a
default, and an object URL that is never revoked. The held-out set is for measuring only: prompts
are not adjusted on it.

### ReviewBench, the test pull requests {#reviewbench-pilot}

On 12 October 2026 the benchmark's 25 test pull requests were reviewed once by the panel and once by
the single agent, every agent on `opencode-go/deepseek-v4.1-flash`, and scored by the benchmark's
own judge on `deepseek-v4-flash` in place of the Claude Sonnet 5 its leaderboard uses. The run
stopped when both model accounts ran out: ten pull requests have a score for both configurations,
the four largest have no review, and every comment was handed in, the nitpicks included.

| Reviewer                  | Grounded recall | Grounded precision | Comments a pull request | Tokens (thousands) |
| ------------------------- | --------------- | ------------------ | ----------------------- | ------------------ |
| eyeful panel              | 0.41            | 0.82               | 16.0                    | 590                |
| eyeful panel, first five  | 0.26            |                    | 4.5                     |                    |
| eyeful panel, first three | 0.19            |                    | 2.9                     |                    |
| eyeful single agent       | 0.25            | 0.79               | 6.2                     | 190                |

The panel found more golden findings than the single agent on seven of the ten pull requests and
fewer on none. On the leaderboard, where recall follows the number of comments, Codex at its best
has 0.185 with 3.4 comments, Devin 0.238 with 5.0 and the first entry 0.260 with 5.4, all with a
precision between 0.84 and 0.90. Counted at the same number of comments the panel is level with
them and not ahead, and this judge is lenient: it called 93 of the panel's 104 unmatched comments
real problems. The comparison with the leaderboard therefore waits for a run judged by Sonnet 5.

What the scores show about the panel itself does not depend on the judge's level:

- The order of the comments is sound at the top and the tail is long. 55% of the first five
  comments of a pull request match a golden finding, 25% of the sixth to the tenth and 15% of the
  rest, which is half of all comments.
- Agreement predicts a match. A comment two or more experts reported matches 62% of the time, one
  from a single expert 19%.
- The judge's objection is informative. None of the 11 comments it disputed matched a golden
  finding, and they are still shown.
- Nitpicks are 76 of the 160 comments and 16% of them match.
- The gain over the single agent is in medium and low findings (0.55 against 0.38, and 0.27
  against 0.09). On high-severity findings the panel found 5 of 11 and the single agent 6.

The runs also found two defects, both fixed with a test: a diff with a file renamed and not changed
failed in planning, because `@pulls.review/core` hashes such a file with a function the embedded
runtime did not have; and a review without findings wrote its verification list as `null`.
