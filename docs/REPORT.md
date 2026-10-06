# Review feedback

The summarizer is the last agent in a review. It receives every finding and every verification
result and writes comments a person can act on. `workflow` merges, checks and ranks the comments
and falls back to rules, and `workflow/report` renders them as Markdown and SARIF.

## Comment format {#comment-format}

Every comment follows [Conventional Comments](https://conventionalcomments.org):

```text
<label> [decorations]: <subject>

[discussion]
```

The label gives the kind of comment, and the decorations say whether it blocks and which area it
concerns. The subject is one sentence, and the discussion explains why and what to do next. With a
fixed format, the same comment can be posted to GitHub, printed in a terminal, or parsed by another
agent.

eyeful uses all nine labels the specification recommends:

| Label        | Meaning in the specification                        | eyeful uses it for                                                        |
| ------------ | --------------------------------------------------- | ------------------------------------------------------------------------- |
| `praise`     | Something positive, sincerely meant                 | A clear improvement an expert noticed                                     |
| `nitpick`    | A trivial, preference-based request; non-blocking   | A style point the project's style guide does not cover                    |
| `suggestion` | A proposed improvement, explicit about what and why | An improvement, with a patch when the fix is clear                        |
| `issue`      | A specific problem, best paired with a suggestion   | A problem with strong evidence, or a rule the style guide states          |
| `todo`       | A small, trivial but necessary change               | A version string, a changelog line, a missing export                      |
| `question`   | A potential concern the reviewer is not sure about  | A concern an expert could not prove                                       |
| `thought`    | An idea from reviewing; non-blocking                | A design direction worth considering later                                |
| `chore`      | A process task needed before acceptance             | A step the project's process requires, such as updating docs, with a link |
| `note`       | Something the reader should notice; non-blocking    | Context such as a dependency whose licence changed                        |

Decorations also follow the specification. `blocking` and `non-blocking` say whether the comment
has to be resolved before merging, and `if-minor` asks for a change only if it turns out to be
small. Only an `issue` with strong evidence of a bug, a vulnerability or a regression is `blocking`
([A review](REVIEW.md#review-standard)). Everything else is `non-blocking`.

Other decorations name the area: `security`, `correctness`, `design`, `tests`, `ux`, `readability`.
As Google's [comment guidance](https://google.github.io/eng-practices/review/reviewer/comments.html)
recommends, comments are about the code and not the author, and each discussion gives the reason.
The summarizer writes each comment from the finding's SARIF result
([Evidence and verification](VERIFICATION.md#sarif)).

```text
issue (blocking, correctness): A session is still valid at exactly expires_at.

is_expired compares with >, so a request at the expiry second is accepted. The reproduction
test below fails on this change and passes with the suggested fix; the rest of the suite stays
green.
```

```text
nitpick (non-blocking, readability): `t` could say what it holds, such as `expires_at`.
```

## Line comments

eyeful posts its comments as an ordinary pull request review with the event `COMMENT`. It does not
use `REQUEST_CHANGES` or `APPROVE`. Each `issue (blocking)` is placed on the line of the finding,
and a verified fix is attached as a GitHub suggested change that the author can apply with one
click. The label tells the author what has to change, but the review does not stop a merge. Once the
issues are fixed nothing is left to dismiss, and a person merges.

A repository that wants a hard gate can enable an optional Check Run, which is off by default. It
fails while the review has any `issue (blocking)` and passes otherwise. A Check Run belongs to a
commit, so a new push and its re-review replace it, and a branch protection rule can require it.

## The ranked report {#ranked-report}

All other comments go into one report, posted as a single pull request comment and also kept as
Markdown and SARIF. Findings are merged first: findings in the same file within two lines of each
other become one, and corroboration counts how many distinct experts reported it. Experts on the same
model family tend to make the same mistakes, so agreement between different model families counts
for more than agreement within one family. The merged findings are then sorted by:

1. label: `issue`, `todo`, `chore`, `suggestion`, `question`, `thought`, `note`, `nitpick`, `praise`;
2. evidence strength ([Evidence and verification](VERIFICATION.md#evidence-kinds));
3. the number of experts that reported it;
4. the plan's change groups, then file and line.

All `nitpick` comments are folded into one collapsed section at the end. The report closes with:

- for a change split into [scopes](PLANNER.md#large), the scopes in order with their files, lines
  and findings, and each finding names its scope;
- the files triage set aside, listed as not reviewed;
- the uncertainty checklist: absent experts, skills an expert was offered and did not load, findings
  that could not be reproduced, commands and suites that did not run, and whether the budget ran
  out.

## Outputs

| Output         | Where                                                                        |
| -------------- | ---------------------------------------------------------------------------- |
| GitHub review  | Comments on lines, as a `COMMENT` review (cloud)                             |
| GitHub comment | The ranked report on the pull request (cloud)                                |
| Check Run      | Optional, off by default: fails while any `issue (blocking)` remains (cloud) |
| Markdown       | The console and the terminal; both parts, blocking first                     |
| Findings JSON  | The desktop app: the findings beside the diff and on their lines             |
| SARIF          | Another agent, a script, GitHub code scanning, the evaluation                |

The GitHub outputs and the console view are planned.

## In the terminal {#terminal}

`eyeful review` presents findings the way Claude Code's ultrareview does. It opens with the counts
and, as pulls.review does before any comment, what the change does: the planner's summary and a
table of its groups, core groups first, each with its category, why its files changed and the files.
Then comes "No blocking issues" or a table of findings sorted by severity:

```text
| Severity     | File:Line            | Issue                                     |
| ------------ | -------------------- | ----------------------------------------- |
| 🔴 Important | `app/session.py:11`  | Session still valid at exactly expires_at |
| 🟡 Nit       | `web/src/Login.svelte:40` | Name the expiry variable             |
```

A blocking `issue` is 🔴 Important and every other label is 🟡 Nit. Each finding then gets a
heading with its severity, its area and a verdict: CONFIRMED when its evidence is strong, such as a
reproduction eyeful saw fail with an outside oracle, and PLAUSIBLE otherwise. Below the heading come
the one-sentence summary, the failure scenario (the input that triggers the problem and what goes
wrong), a collapsed "Why this was flagged" with the reasoning, how eyeful verified it and the
reproduction test, and the verified fix as a diff. The files triage set aside and the uncertainty
checklist close the output.

Each run also saves the same findings as `findings.json`, with the field names of Claude Code's
findings list (`file`, `line`, `summary`, `short_summary`, `failure_scenario`, `category`,
`verdict`) plus `severity`, along with `review.md`,
`findings.json`, `results.sarif` and the full `result.json` ([Running locally](LOCAL.md)).

Outputs are sinks plugged in at the end of the workflow ([Architecture](ARCHITECTURE.md#data-flow)),
so adding an output means adding a sink.

## Rounds

A change usually goes through several rounds of review and fixes. A re-review is a new review of the
same subject. It receives the previous round's findings but not its reasoning. Each expert is asked
to judge those findings again rather than confirm them, and a blocking issue that has been fixed is
marked resolved in the new review.

## When summarizing fails

The summarizer writes the label, subject and discussion of each merged finding; Go adds the
decorations, `blocking` only on an `issue` with strong evidence, and the area of each expert behind
it. Go rejects the summary when a merged finding does not get exactly one comment, a label is not
one of the nine, or an `issue` lacks strong evidence.

If the summarizer fails or its summary is rejected, the feedback is formatted by rules instead.
Findings are labelled by evidence strength alone, `issue` for strong evidence and `question` for the
rest, and grouped by file. No finding is dropped.
