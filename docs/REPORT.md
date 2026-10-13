# Review feedback

The last stage turns findings and verification results into comments. No agent is involved:
`workflow` merges the findings that are [one problem](VERIFICATION.md#one-problem), labels and ranks
them by rule, and `workflow/report` renders them as Markdown and SARIF.

## Comment format {#comment-format}

Every comment follows [Conventional Comments](https://conventionalcomments.org):

```text
<label> [decorations]: <subject>

[discussion]
```

A label follows from three facts about a finding, each on a published scale:

| Fact                  | Scale                                             | In eyeful                                        |
| --------------------- | ------------------------------------------------- | ------------------------------------------------ |
| What it is about      | Functional or evolvability, Mäntylä and Lassenius | A `note` is evolvability; the rest is functional |
| How it can be checked | Verification methods, ISO/IEC/IEEE 29148          | [Evidence](VERIFICATION.md#evidence-kinds)       |
| How settled it is     | `kind`, SARIF 2.1.0                               | [Kind](VERIFICATION.md#kind-and-severity)        |

Go takes the first row that fits. The names and their meanings are those of Conventional Comments;
which facts give which label is eyeful's own rule ([References](REFERENCES.md#code-review)).

| Label        | Given to                                                                     | Conventional Comments                                       |
| ------------ | ---------------------------------------------------------------------------- | ----------------------------------------------------------- |
| `nitpick`    | Every finding behind the comment is a `note`                                 | "trivial preference-based requests"                         |
| `issue`      | A finding whose kind is `fail`                                               | "highlight specific problems"                               |
| `question`   | A finding that names a failing scenario, not yet shown: `test` or `analysis` | "a potential concern but … not quite sure if it's relevant" |
| `suggestion` | A finding with no scenario, read against a standard: `inspection`            | "propose improvements"                                      |

Decorations say whether the comment blocks and which area it concerns: `security`, `correctness`,
`design`, `tests`, `ux`, `readability`. Only an `issue` is `blocking`
([A review](REVIEW.md#review-standard)). The subject and discussion are the expert's own words,
taken from the most severe finding behind the comment.

```text
issue (blocking, correctness): A session is still valid at exactly expires_at.

is_expired compares with >, so a request at the expiry second is accepted. The reproduction
test below fails on this change and passes with the suggested fix.
```

## Line comments

Planned for the cloud. eyeful posts a pull request review with the event `COMMENT`, never
`REQUEST_CHANGES` or `APPROVE`. Each `issue (blocking)` sits on its line, with a verified fix as a
GitHub suggested change. A repository that wants a gate can enable a Check Run, off by default,
which fails while any `issue (blocking)` remains.

## The ranked report {#ranked-report}

Everything else goes into one report. Findings that are [one problem](VERIFICATION.md#one-problem)
are merged, and the comment records how many experts reported it. Comments are sorted by:

1. standing: not disputed by the judge, then disputed, then nitpicks;
2. kind: `fail`, `review`, `open` ([Kind and severity](VERIFICATION.md#kind-and-severity));
3. the number of experts that reported it;
4. label: `issue`, `question`, `suggestion`, `nitpick`;
5. the plan's groups, then file and line.

Disputed comments and nitpicks are folded into collapsed sections at the end. Nothing is left out.
The report closes with:

- the [scopes](PLANNER.md#large) of a split change, with their files and findings;
- the files triage set aside;
- the uncertainty checklist: absent experts, skills not loaded, findings not reproduced, commands
  that did not run, and whether the budget ran out.

## Outputs

| Output         | For                                                           |
| -------------- | ------------------------------------------------------------- |
| Markdown       | The terminal and the console                                  |
| Findings JSON  | The desktop app, beside the diff                              |
| SARIF          | Another agent, a script, GitHub code scanning, the evaluation |
| GitHub review  | Line comments (planned)                                       |
| GitHub comment | The ranked report (planned)                                   |
| Check Run      | An optional gate (planned)                                    |

Outputs are sinks at the end of the workflow ([Architecture](ARCHITECTURE.md#data-flow)).

## In the terminal {#terminal}

`eyeful review` prints, in order:

1. the counts of findings by kind;
2. what the change does: the planner's summary and its groups, core groups first;
3. the tests the change breaks ([Regressions](VERIFICATION.md#regressions));
4. "No blocking issues", or a table of findings in report order:

```text
| Severity     | File:Line                 | Issue                                     |
| ------------ | ------------------------- | ----------------------------------------- |
| 🔴 error     | `app/session.py:11`       | Session still valid at exactly expires_at |
| 🟠 warning   | `web/src/Login.svelte:40` | The expiry is shown in UTC                |
```

5. each finding: its severity, area and kind, a one-sentence summary, the failure scenario, the
   reasoning, how it was verified, the reproduction test and the fix as a diff;
6. the disputed comments, the nitpicks, the files set aside and the uncertainty checklist.

Each run saves `review.md`, `findings.json`, `results.sarif` and `result.json`
([Running locally](LOCAL.md)).

## Rounds {#rounds}

A re-review is a new review of the same subject: locally, the latest run in `.eyeful/runs` with the
same base commit whose head is the current head or an ancestor of it. With `execution`, before any
expert starts, the verifier runs every reproduction the last review saw fail, five times each, with
no model:

| The reproduction now                                | The finding                                                      |
| --------------------------------------------------- | ---------------------------------------------------------------- |
| passes every time                                   | Resolved, and listed under "Resolved since the last review"      |
| fails with its `fails` text, not on the base commit | Still open; carried over with its test, and the experts are told |
| fails the same way on the base commit               | Pre-existing; reported, never blocking                           |
| is flaky, fails another way or cannot run           | Shown as unverified and listed in the uncertainty checklist      |

Findings the last review did not reproduce are not carried: the experts read the change afresh.
With `read_only` or `none` nothing is carried.
