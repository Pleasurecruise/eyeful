# Review feedback

The last stage of a review writes comments a person can act on from every finding and every
verification result. No agent is involved: `workflow` merges the findings, forms one comment for
each from what its expert wrote, and ranks them, and `workflow/report` renders them as Markdown and
SARIF.

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

eyeful uses four of the specification's labels, and Go picks the label by rule:

| Label        | Meaning in the specification                        | eyeful gives it to                                                         |
| ------------ | --------------------------------------------------- | -------------------------------------------------------------------------- |
| `issue`      | A specific problem, best paired with a suggestion   | A finding whose kind is `fail`                                             |
| `question`   | A potential concern the reviewer is not sure about  | A finding whose evidence is `test` or `analysis`, not yet a `fail`         |
| `suggestion` | A proposed improvement, explicit about what and why | A finding that rests on `inspection` alone                                 |
| `nitpick`    | A trivial, preference-based request; non-blocking   | A `note` finding: one that changes no behaviour, such as a name or wording |

The other five labels of the specification, `praise`, `todo`, `thought`, `chore` and `note`, are
not used: nothing an expert hands in tells them apart.

### Why no agent writes the comments {#no-summarizer}

A summarizer agent used to write every comment: it read the merged findings, chose one of the nine
labels and rewrote the subject and the discussion. It was removed, for four reasons:

- Its label had stopped deciding anything. Go already overrode it in both directions, allowing
  `issue` only on a `fail` result and forcing `nitpick` on `note` findings, and the ranking moved
  to evidence and agreement, which left the label fourth.
- It could not check what it wrote. It had no file tools and saw only the findings, so its text
  could only restate the expert's, or drift from it.
- There were two ways to form a comment. When the agent failed or its answer was rejected, rules
  formed the comments instead, so the rules had to exist anyway and a review could come out either
  way.
- It cost a model call on every review: about 2% of the tokens, the time of one more agent, and one
  more thing that could fail.

What this gave up is real and has not been measured. A comment that several experts stand behind
now carries the words of one of them, where the agent could combine them; the wording is the
expert's own, unedited; and five labels are gone. No review has been scored since the change, so
whether the comments read worse, or match a benchmark's findings less often, is not known.

Decorations also follow the specification. `blocking` and `non-blocking` say whether the comment
has to be resolved before merging; the specification's third, `if-minor`, is not used, because
nothing an expert hands in says how large a fix would be. Only an `issue`, a `fail` result that shows a bug, a vulnerability or a regression, is `blocking`
([A review](REVIEW.md#review-standard)). Everything else is `non-blocking`.

Other decorations name the area: `security`, `correctness`, `design`, `tests`, `ux`, `readability`.
As Google's [comment guidance](https://google.github.io/eng-practices/review/reviewer/comments.html)
recommends, comments are about the code and not the author, and each discussion gives the reason.
The subject and the discussion of a comment are the expert's own, taken from the most severe
finding behind it.

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
Markdown and SARIF. Findings are merged first: findings of different experts in the same file within
two lines of each other become one, and corroboration counts how many distinct experts reported it.
Two findings of one expert are two problems however close they are, so each keeps its own comment.
Experts on the same
model family tend to make the same mistakes, so agreement between different model families counts
for more than agreement within one family. The merged findings are then sorted by:

1. what the checks left standing: comments the judge did not dispute, then the ones it disputed,
   then the nitpicks;
2. kind: `fail`, then `review`, then `open` ([Evidence and verification](VERIFICATION.md#kind-and-severity));
3. the number of experts that reported it;
4. label: `issue`, `question`, `suggestion`, `nitpick`;
5. the plan's change groups, then file and line.

Evidence and agreement come before the label because they are measured and the label is derived
from them: on ReviewBench's test pull requests a comment two experts reported
matched a known finding three times as often as one from a single expert, and none of the comments
the judge disputed matched one ([Evaluation](EVALUATION.md#reviewbench-pilot)). An `issue` still
comes first, since only a `fail` result can be one.

Two kinds of comment are folded into collapsed sections at the end, one line each: the comments the
judge disputes, each with its objection, and the `nitpick` comments. A comment is a `nitpick` when
every finding behind it has severity `note`. Nothing is left
out of the report; folding decides what the reader meets first. The report closes with:

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
The project's tests that pass on the base commit and fail on the change are listed next
([Tests the change breaks](VERIFICATION.md#regressions)). Then comes "No blocking issues" or a table
of findings in the order of the ranked report, without the nitpicks:

```text
| Severity     | File:Line            | Issue                                     |
| ------------ | -------------------- | ----------------------------------------- |
| 🔴 error     | `app/session.py:11`  | Session still valid at exactly expires_at |
| 🟠 warning   | `web/src/Login.svelte:40` | The expiry is shown in UTC            |
```

The severity is the expert's, in SARIF's levels: 🔴 error, 🟠 warning or 🟡 note. Each finding then
gets a heading with its severity, its area and its kind: `fail` when eyeful saw its reproduction
fail by an oracle that is not the agent's, and `review` otherwise
([Kind and severity](VERIFICATION.md#kind-and-severity)). The first line counts the findings by
kind. A finding the judge
rejected ([the judge](VERIFICATION.md#the-judge)) gets no heading: it is folded under "Disputed". Below the heading come
the one-sentence summary, the failure scenario (the input that triggers the problem and what goes
wrong), a collapsed "Why this was flagged" with the reasoning, how eyeful verified it and the
reproduction test, and the verified fix as a diff. The folded nitpicks, the files triage set aside
and the uncertainty checklist close the output.

Each run also saves the same findings as `findings.json`, with the field names of Claude Code's
findings list (`file`, `line`, `related`, `summary`, `short_summary`, `failure_scenario`, `category`)
plus SARIF's `severity` and `kind`, along with `review.md`,
`findings.json`, `results.sarif` and the full `result.json` ([Running locally](LOCAL.md)).

Outputs are sinks plugged in at the end of the workflow ([Architecture](ARCHITECTURE.md#data-flow)),
so adding an output means adding a sink.

## Rounds {#rounds}

A change usually goes through several rounds of review and fixes. One review is an event: it gives a
verdict on one diff and is over. What a person needs between rounds is a state, namely which
problems are still there, and eyeful keeps that state as the reproductions it saw fail, not as a
memory of what its agents said.

A re-review is a new review of the same subject. Locally that is the latest earlier run in
`.eyeful/runs` that compared against the same base commit and whose head is the head reviewed now
or an ancestor of it; for uncommitted changes the same base is enough. A review of another change
that merely starts where an earlier one ended has a different base and carries nothing over. With `execution`, before any expert starts, the verifier runs
again every reproduction the last review saw fail, five times each, and no model is called:

| The reproduction now                                    | The finding                                                                                                                             |
| ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------- |
| passes every time                                       | Resolved. The report lists it under "Resolved since the last review" with its test, and it gets no comment                              |
| fails with its `fails` text, and not on the base commit | Still open. It is carried into this review with the same test and oracle as evidence, and the experts are told it is already found      |
| fails the same way on the base commit                   | Pre-existing: the problem was merged, so this change no longer introduces it. Reported, never blocking                                  |
| is flaky, fails another way or cannot run               | Shown again as an unverified comment and listed in the uncertainty checklist; a file now at the test's path is one reason it cannot run |

A finding counts as fixed only when its test went from red to green in front of the verifier. A
finding the last review graded on a scale this version does not have, an oracle or a severity it
does not know, is not carried, and neither is one it did not reproduce: the experts read the change afresh and
report it again if it is still there. With `read_only` or `none` nothing runs, so nothing is carried
either. Handing the last review's unproven findings to the judge again is planned.
