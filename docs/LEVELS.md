# Levels

A review should cost about as much as the change needs. To get there, eyeful starts each agent only
when a cheaper check has found something for it. Rules run first, then tools, then cheap models,
and the experts last. Each layer looks only at what the layer before it flagged. The level of a
review sets the highest layer it may reach. `workflow` implements level choice, the routing rules,
the expert limits and the budget; the L1 tools themselves are not built yet.

## The four layers

```mermaid
flowchart TB
    l0["L0 Rules<br/>CI check, triage, risk paths"] -->|signals| l1["L1 Tools<br/>lint, typecheck, audits, diffs"]
    l1 -->|findings| l2["L2 Cheap models<br/>planner, readability"]
    l2 -->|plan| l3["L3 Experts<br/>mid and strong models, verification"]
```

| Layer | Runs                                                                                                  | Model cost |
| ----- | ----------------------------------------------------------------------------------------------------- | ---------- |
| L0    | The CI check, triage, matching `risk` paths and file types                                            | none       |
| L1    | `lint`, `typecheck`, `secret_scan`, `dependency_audit`, `openapi_diff`, `complexity`, `coverage_diff` | none       |
| L2    | The planner, the readability expert, turning tool output into comments                                | small      |
| L3    | The correctness, security, design, tests and usability experts, and reproduction                      | most       |

L0 and L1 are deterministic and use no tokens, so every review runs them. Their output is also what
the upper layers act on.

## Levels

| Level    | Always runs     | Started when a signal calls for it                                     | Verification                                                          |
| -------- | --------------- | ---------------------------------------------------------------------- | --------------------------------------------------------------------- |
| quick    | L0, L1          | Up to 2 experts on cheap models, only where an L1 tool found something | none                                                                  |
| standard | L0, L1, planner | Up to 4 experts per change group, chosen by the planner                | Reproductions in verification order, plus the touched packages' tests |
| deep     | L0, L1, planner | Any number of experts per group, chosen by the planner, strong models  | Every reproduction, plus the full suite and E2E                       |

The quick level has no planner. Rules send tool output to experts: a `secret_scan` hit to security,
a `typecheck` error to correctness, an `openapi_diff` break to usability. If no tool finds anything,
a quick review is just the list of tool results and uses no tokens. For this reason quick is the one
level that does not require every line to be reviewed ([Planning and triage](PLANNER.md#triage)).

At standard, the planner reads the manifest and the L1 output and picks experts for each change
group. The heavier tools inside an expert also run only on request: `sast`, `e2e_run` and
`a11y_check` run when the expert calls them.

At deep, the planner may give a change group any number of experts, and they run on strong models.
An expert still starts only where the change calls for it, as at standard: the design expert, for
example, is picked for a new module, dependency or layer, a public API signature, or concurrency,
not for every change. Deep spends more on what does run, not on starting every expert.

A few signals start an expert at every level:

| Signal                                         | Effect                  |
| ---------------------------------------------- | ----------------------- |
| A known CVE in an added or upgraded dependency | security runs           |
| A secret in the diff                           | security runs           |
| A breaking change in the API document          | usability runs          |
| A change to a `risk` path                      | the level rises to deep |

## Choosing the level {#choosing-the-level}

Nobody picks the level; eyeful chooses it from the change, the way revmux runs one default profile
instead of asking. A change that touches a `risk` path from
`.eyeful/config.yml` or a `CODEOWNERS` path, or a diff of more than 2,000 changed lines, gets deep, and everything else
gets standard. If the planner reports low confidence, the level goes up; it does not go down. A
review can move up a level while running and reuse what has already run.

Each level has a budget in tokens and money, and each command the verifier runs has its own timeout.
There is no limit on total minutes, since a project's own tests may take longer than any fixed
limit. Go checks the budget before it starts each agent, so agents already running when it runs out
still finish. From then on the review starts no more agents and returns what it has verified, with
result `partial`. Quick is never chosen for a change; the [evaluation](EVALUATION.md) fixes the
level through the workflow's request to measure what standard adds over quick. Deep arrives with the cloud ([Roadmap](ROADMAP.md)).

## Why start agents on demand

Running every expert on every change is simple but expensive. Most changes need two or three
experts, and a linter or an audit already catches many problems. Starting experts only on a signal
keeps the cost in line with the change. Whether this loses findings is measured in the
[evaluation](EVALUATION.md#ablations), which compares it with running every expert.
