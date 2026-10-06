# Product north star

Give every change a full set of reviewers whose findings a maintainer can trust without redoing the
review, because each one comes with evidence that runs.

## Outcomes

- Evidence: strong findings are backed by a reproduction that eyeful itself saw fail before the fix
  and pass after, against an expected behaviour that does not come from the agent (a crash, an
  existing test, or a quoted specification); everything else is labelled with the weaker evidence
  it has.
- Precision: a maintainer acts on what eyeful reports instead of triaging it; false positives fall
  measurably against read-only verification.
- Proportion: a review spends only what the change needs, and each level that has shipped shows what
  its extra time and money buy over the one below it.
- Safety: reviewing an untrusted pull request is as safe as not reviewing it.
- Simplicity: the fewest services, stores, routes and agents that achieve the above.

## Hard measures

- eyeful never approves a change; approval belongs to a person.
- A finding is marked verified only when eyeful's verifier, not the agent that wrote it, ran the
  reproduction and saw it fail.
- No credential (GitHub token, API token, model key) enters a sandbox, a log, an error response or
  the console bundle.
- In the cloud, code under review runs only inside its review's sandbox, which reaches only package
  registries; agents read the code with read-only tools outside it. On a user's machine, only the
  project commands that user confirmed run.
- Local and cloud never mix: no code, record, account or key crosses between a user's machine and
  the cloud.
- A result from a superseded lease is never written; a review is never lost or run twice to
  completion because a worker died.
- A review that runs out of budget still returns what it verified, marked partial.

A change may improve one outcome without a numeric before/after result. Hard measures are release
gates and may never regress. Outcomes are measured as [Evaluation](../docs/EVALUATION.md) describes;
other documents name this file instead of restating a hard measure.
