# Running in the cloud

A cloud review runs on a self-hosted eyeful server against a GitHub pull request. One deployment
serves one GitHub App and has no user accounts. The server and the review queue exist; the webhook
and the review itself are planned.

## What can be reviewed {#what-can-be-reviewed}

A pull request in a repository where eyeful's GitHub App is installed, and nothing else.
Branches, commits and uncommitted changes are reviewed locally ([Running locally](../LOCAL.md)).

## How a cloud review runs

1. A pull request event reaches `POST /webhook`, signed with the App's webhook secret, and queues one
   review per head commit. The project commands come from `.eyeful/config.yml` on the base branch, so
   a pull request cannot change what runs on it.
2. eyeful reads the GitHub Actions runs of the head commit. If one failed, the review ends with a
   diagnosis ([Planning and triage](../PLANNER.md#ci-check)).
3. A worker clones the repository and records the snapshot: the base and head commits and the diff
   against their merge base. A new push is a new snapshot and a new review.
4. Triage runs in the sandbox, and the planner makes a plan.
5. Each expert runs as a pi process on the worker.
6. The verifier runs reproductions on fresh copies in the sandbox.
7. eyeful's GitHub App posts the line comments as one `COMMENT` review and the ranked report as one
   comment ([Review feedback](../REPORT.md)).

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

## Credentials {#credentials}

Planned with the webhook:

| Credential                      | Held by                  | Used for                                 |
| ------------------------------- | ------------------------ | ---------------------------------------- |
| App ID, private key             | The server's environment | Minting an installation token per review |
| Webhook secret                  | The server's environment | Checking `X-Hub-Signature-256`           |
| Installation token, short-lived | The worker, in memory    | Cloning, reading CI, posting the review  |
| Model keys                      | The pi process           | Agent runs, paid by the operator         |

None is stored in the database, written to a log or passed into a sandbox. Each review records its
tokens, cost and minutes.
