# Running in the cloud

A cloud review runs on Cloudflare against a GitHub pull request, with the same workflow as a local
review, in its own Container. The sandbox is not built, so it runs none of the project's commands
and a judge checks its findings.

## What can be reviewed {#what-can-be-reviewed}

A pull request in a repository where eyeful's GitHub App is installed, opened or mentioned by a user
in `EYEFUL_ALLOWED_USERS`. Branches, commits and uncommitted changes are reviewed locally
([Running locally](../LOCAL.md)).

## How a cloud review runs

1. A pull request is reviewed once when it is opened or ready for review, and again each time
   someone comments `@eyeful-review`. When and how often is in [Review lifecycle](SCHEDULING.md).
2. The Worker checks the webhook signature and hands the pull request to its Durable Object, which
   starts the review's Container running `eyeful-server serve`.
3. The server clones the repository, waits for the head commit's GitHub Actions and reacts with 👀.
   A failed run ends the review with a diagnosis ([Planning and triage](../PLANNER.md#ci-check)).
4. The server calls `local/app.Review` with read-only verification, as in
   `eyeful review --verification read_only`, saving each stage to R2 ([Storage](PERSISTENCE.md)).
5. The App posts each undisputed `issue` as a line comment in one `COMMENT` review, and a short
   summary as a comment ([Review feedback](../REPORT.md#line-comments)), then records `published`
   in R2. A crash between the two can post twice; an outbox is planned.

## Agents and models {#agents-and-models}

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
