# Running in the cloud

A cloud review runs on a self-hosted eyeful server against a GitHub pull request. One deployment
serves one GitHub App and has no user accounts. `POST /webhook` queues one review per head commit,
and a worker runs the same workflow as a local review. The sandbox is not built, so a cloud review
does not run the project's commands and its findings stay unverified.

## What can be reviewed {#what-can-be-reviewed}

A pull request in a repository where eyeful's GitHub App is installed, and nothing else.
Branches, commits and uncommitted changes are reviewed locally ([Running locally](../LOCAL.md)).

## How a cloud review runs

1. A pull request event reaches `POST /webhook`, signed with the App's webhook secret, and queues one
   review per head commit. eyeful finds the project commands from the manifests on the base
   branch, so a pull request cannot change what runs on it.
2. eyeful reads the GitHub Actions runs of the head commit. If one failed, the review ends with a
   diagnosis ([Planning and triage](../PLANNER.md#ci-check)).
3. A worker clones the repository and records the snapshot: the base and head commits and the diff
   against their merge base. A new push is a new snapshot and a new review.
4. Triage runs on the worker, and the planner makes a plan. The commands eyeful found on the base
   commit are recorded on the review. They do not run.
5. Each expert runs as one process of the agent named by `EYEFUL_AGENT` (pi, unless that is set),
   with read-only tools over the checkout.
6. The verifier would run reproductions on fresh copies in the sandbox. That sandbox is not built,
   so setup records that nothing was verified and the findings stay unverified.
7. eyeful's GitHub App posts the line comments as one `COMMENT` review and the ranked report as one
   comment ([Review feedback](../REPORT.md)). A file named `published` next to the stage archive
   records that GitHub accepted both. A retry that finds it returns without posting again. A crash
   after GitHub accepts the posts and before that file is written can post them twice. An outbox
   is planned.

## Agents in pi {#agents-in-pi}

Each agent is a process with a fresh `HOME`, no saved session, and read-only tools over the
checkout. `GITHUB_TOKEN`, `GH_TOKEN` and every `EYEFUL_` variable are removed from its environment.
Instructions committed to the reviewed repository are ignored. The model keys stay in that
process, outside the sandbox. For pi the command is:

```sh
pi --mode json --no-session \
  --tools read,grep,find,ls \
  --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve
```

The instructions are the prompt on stdin. `--tools` is this fixed list; the extension tools named
in an expert file are planned. Provider routing is built and not wired in
([Agent runs over providers](SCHEDULING.md#agent-runs-over-providers)), so every role uses the one
agent named by `EYEFUL_AGENT`.

## The sandbox {#the-sandbox}

These limits are hard measures in `.agents/POLARIS.md`. The sandbox they describe is not built.
Until it is, the worker refuses `setup` and every project command, and it says so in the report.

- Each review gets one sandbox.
- No credential enters it: no GitHub token, API key or model key.
- It reaches package registries during `setup` and nothing else.
- Each reproduction runs on a fresh copy.
- Sandboxes are labelled with their review, and the server removes orphans when it starts.

## Credentials {#credentials}

| Credential                      | Held by                  | Used for                                 |
| ------------------------------- | ------------------------ | ---------------------------------------- |
| App ID, private key             | The server's environment | Minting an installation token per review |
| D1 API token                    | The server's environment | Querying D1                              |
| Webhook secret                  | The server's environment | Checking `X-Hub-Signature-256`           |
| Installation token, short-lived | The worker, in memory    | Cloning, reading CI, posting the review  |
| Model keys                      | The pi process           | Agent runs, paid by the operator         |

None is stored in the database, written to a log or passed into a sandbox. Each review records its
tokens, cost and minutes.
