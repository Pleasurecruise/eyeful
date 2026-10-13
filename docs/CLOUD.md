# Running in the cloud

A cloud review runs on the eyeful server against a GitHub pull request. The server, sign-in and the
review queue exist. The review itself is planned.

## What can be reviewed {#what-can-be-reviewed}

A pull request in a repository the user has given eyeful's GitHub App access to, and nothing else.
Branches, commits and uncommitted changes are reviewed locally ([Running locally](LOCAL.md)).

## How a cloud review runs

1. The user confirms the review in the console ([A review](REVIEW.md#before-it-starts)).
2. eyeful reads the GitHub Actions runs of the head commit. If one failed, the review ends with a
   diagnosis ([Planning and triage](PLANNER.md#ci-check)).
3. A worker clones the repository and records the snapshot: the base and head commits and the diff
   against their merge base. A new push is a new snapshot and a new review.
4. Triage runs in the sandbox, and the planner makes a plan.
5. Each expert runs as a pi process on the worker.
6. The verifier runs reproductions on fresh copies in the sandbox.
7. eyeful's GitHub App posts the line comments as one `COMMENT` review and the ranked report as one
   comment ([Review feedback](REPORT.md)).

## Agents in pi {#agents-in-pi}

Each agent is a pi process with a fresh `HOME`, no saved session, and read-only tools over the
checkout. Instructions committed to the reviewed repository are ignored. The model keys stay with
the pi process, outside the sandbox.

```sh
pi --print --mode json --no-session \
  --model anthropic/claude-sonnet-5-5 \
  --tools read,grep,find_references,secret_scan,dependency_audit,cve_lookup,sast,submit_report \
  --extension eyeful-pi-extension.js \
  --append-system-prompt workflow/prompts/experts/security.md \
  "Review the files the plan assigned to you"
```

`--tools` limits the agent to what its expert file allows. The extension provides those tools; each
call runs a fixed command in the sandbox. The model comes from
[provider routing](SCHEDULING.md#agent-runs-over-providers).

## The sandbox {#the-sandbox}

- Each review gets one sandbox.
- No credential enters it: no GitHub token, API key or model key.
- It reaches package registries during `setup` and nothing else.
- Each reproduction runs on a fresh copy.
- Sandboxes are labelled with their review, and the server removes orphans when it starts.

These limits are hard measures in `.agents/POLARIS.md`.

## Accounts and cost

A cloud review belongs to the user who created it ([Sign-in and credentials](AUTH.md)). Model usage
is paid with the server's keys, and each review records its tokens, cost and minutes.
