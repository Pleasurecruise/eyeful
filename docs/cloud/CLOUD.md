# Running in the cloud

A cloud review runs on Cloudflare against a GitHub pull request. One deployment serves one GitHub
App and has no user accounts. Each pull request head commit gets one review, and each review runs
the same workflow as a local review, in its own Container. The sandbox is not built, so a cloud
review does not run the project's commands and its findings are checked by a judge.

## What can be reviewed {#what-can-be-reviewed}

A pull request in a repository where eyeful's GitHub App is installed, and nothing else. Branches,
commits and uncommitted changes are reviewed locally ([Running locally](../LOCAL.md)).

## How a cloud review runs

1. GitHub sends a `pull_request` event (`opened`, `synchronize`, `reopened` or `ready_for_review`)
   to the Worker. The Worker checks `X-Hub-Signature-256` with `@octokit/webhooks`, ignores drafts
   until they are ready for review, and answers `202` at once.
2. The Worker hands the pull request to the Durable Object named by the repository, the number and
   the head commit, so a delivery sent twice reaches the same review
   ([Review lifecycle](SCHEDULING.md)).
3. The Durable Object starts the review's Container, which runs `eyeful-server serve` with the pull
   request in `EYEFUL_REVIEW`.
4. The server mints an installation token, clones the repository and waits until the GitHub Actions
   runs of the head commit finish. If one failed, the review ends with a diagnosis
   ([Planning and triage](../PLANNER.md#ci-check)).
5. The server calls `local/app.Review` with read-only verification: triage, the plan, the experts,
   the judge and the report run exactly as in `eyeful review --verification read_only`. Each stage
   is saved to R2, so a Container started again resumes from the last finished stage
   ([Storage](PERSISTENCE.md)).
6. eyeful's GitHub App posts the line comments as one `COMMENT` review and the ranked report as one
   comment ([Review feedback](../REPORT.md)), then records `published` in R2. A review started again
   after that returns without posting. A crash after GitHub accepts the posts and before the record
   is written can post them twice; an outbox is planned.

## Agents and models {#agents-in-pi}

Each expert is one pi process in the Container, with read-only tools over the checkout:

```sh
pi --mode json --no-session \
  --tools read,grep,find,ls \
  --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve
```

The image carries pi's `settings.json` and `models.json` from `packages/worker/pi/`: the model is
DeepSeek V4.1 Flash on the `volcengine` custom provider of the account's `default` AI Gateway. pi
calls `http://models.eyeful`, which the Durable Object routes to the Worker's `Models` entrypoint;
it forwards the request to AI Gateway with `cf-aig-authorization`. The Container holds no model key
and no gateway token.

[Jev](../ARCHITECTURE.md#jev) runs as `typesafe/jev` on Workers AI. The server's classifier calls
`http://jev.eyeful`, and the Worker's `Jev` entrypoint answers through its AI binding and the same
gateway.

## The sandbox {#the-sandbox}

These limits are hard measures in `.agents/POLARIS.md`. The sandbox they describe is not built.
Until it is, a cloud review runs no project command and verifies by a judge, and the report says so.

- Each review gets one sandbox.
- No credential enters it: no GitHub token, API key or model key.
- It reaches package registries during `setup` and nothing else.
- Each reproduction runs on a fresh copy.

## Credentials {#credentials}

| Credential                   | Held by                                            | Used for                                 |
| ---------------------------- | -------------------------------------------------- | ---------------------------------------- |
| Webhook secret               | The Worker                                         | Checking `X-Hub-Signature-256`           |
| App ID, private key          | The Worker; passed to the Container when it starts | Minting an installation token per review |
| Installation token           | The server, in memory                              | Cloning, reading CI, posting the review  |
| AI Gateway token, account ID | The Worker                                         | The `Models` entrypoint                  |
| Provider keys                | AI Gateway, in the account's Secrets Store         | Upstream model calls                     |
| R2 and Workers AI            | The Worker's bindings                              | Stages, Jev                              |

The server removes every `EYEFUL_` variable from its own environment once it has read its
configuration, so git and pi do not inherit the App key. The Container's start environment still
holds it, and an agent's `read` tool could reach `/proc/1/environ`.
TODO(security): fetch the App key into memory instead of passing it in the start environment.
