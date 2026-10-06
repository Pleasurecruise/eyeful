# Running in the cloud

A cloud review runs on the eyeful server against a GitHub pull request. Agents use model keys configured on the server, and the project's commands run in a
sandbox created for each review. The server, sign-in and the review queue exist today; the review
itself is planned.

## What can be reviewed {#what-can-be-reviewed}

A pull request in a repository that the user has given eyeful's GitHub App access to, and nothing
else: no branch, commit or uncommitted change. The cloud does not read anything from a user's
machine; those are reviewed locally ([Running locally](LOCAL.md)).

## How a cloud review runs

1. The user confirms the review in the console, as described in
   [A review](REVIEW.md#before-it-starts). Webhook and scheduled sources come later
   ([Roadmap](ROADMAP.md)).
2. eyeful reads the GitHub Actions workflow runs for the head commit and waits while any is still
   running. If one failed, the CI agent reads the failed jobs' logs, and the review ends with a
   diagnosis ([Planning and triage](PLANNER.md#ci-check)).
3. A worker clones the repository, resolves the pull request's base and head to commits, and records
   the snapshot (both commits and the diff of head against their merge base) before anything runs.
   A new push is a new snapshot and a new review. A large pull request is split into
   [scopes](PLANNER.md#large) like any change, and the report's list of scopes is the suggested
   stack for it.
4. Triage runs the rules and the L1 tools in the sandbox, and from standard up the planner makes a
   plan.
5. Each selected expert runs as a pi process on the worker.
6. The verifier runs reproductions on fresh copies in the sandbox, and the summarizer writes the
   feedback.
7. eyeful's GitHub App posts the line comments as one `COMMENT` review and the ranked report as one
   comment, plus the optional Check Run. The SARIF results can also go to code scanning
   ([Review feedback](REPORT.md)).

## Agents in pi {#agents-in-pi}

Each agent is a pi process with a fresh `HOME`, no saved session, and read-only tools over the
checkout. Instructions committed to the reviewed repository are ignored. The model keys stay with
the pi process, outside the sandbox, so they never come near the code under review. An expert is
started roughly like this:

```sh
pi --print --mode json --no-session \
  --model anthropic/claude-sonnet-5-5 \
  --tools read,grep,find_references,secret_scan,dependency_audit,cve_lookup,sast,submit_report \
  --extension eyeful-pi-extension.js \
  --append-system-prompt workflow/prompts/experts/security.md \
  "Review the files the plan assigned to you"
```

`--mode json` streams events for Go to parse, and `--tools` limits the agent to the tools its expert
file allows. The extension provides the expert tools, `submit_plan` and `submit_report`. Each tool call runs eyeful's fixed command in the sandbox and returns a result with
a fixed shape. The model for each role comes from
[provider routing](SCHEDULING.md#agent-runs-over-providers).

## The sandbox {#the-sandbox}

Each review gets one sandbox. Reviewing the same subject again is a new review with a new sandbox.
No credential enters the sandbox: no GitHub token, API key or model key. It can reach package
registries during `setup` and nothing else. Each reproduction runs on a fresh copy, so one finding's
test cannot affect another's. Sandboxes are labelled with their review, and the server removes
orphaned ones when it starts. These limits are hard measures in `.agents/POLARIS.md`.

## Accounts and cost

A cloud review belongs to the user who created it ([Sign-in and credentials](AUTH.md)). Model usage
is paid with the keys configured on the server, and each review records its tokens, cost and
minutes.
