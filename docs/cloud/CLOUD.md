# Running in the cloud

A cloud review runs in a GitHub Actions job against a pull request, with the same workflow as a local
review. The agents run in one container and the project's tests in another, the sandbox.

## What can be reviewed {#what-can-be-reviewed}

A pull request in a repository whose workflows use the eyeful action, opened or mentioned by its
owner, a member or a collaborator. Branches, commits and uncommitted changes are reviewed locally
([Running locally](../LOCAL.md)).

## How a cloud review runs

1. A pull request is reviewed once when it is opened or marked ready, and again each time someone
   comments `@eyeful-review`. When a review runs is in [Review lifecycle](SCHEDULING.md).
2. The job builds two images, starts the sandbox, and runs `eyeful-action` in the review container.
3. `eyeful-action` reads the pull request, clones it, waits for the head commit's other workflow
   runs and reacts with 👀. A failed run ends the review with a diagnosis
   ([Planning and triage](../PLANNER.md#ci-check)).
4. It opens the sandbox and calls `local/app.Review` with execution verification.
5. It posts each undisputed `issue` as a line comment in one `COMMENT` review, and a short summary as
   a comment ([Review feedback](../REPORT.md#line-comments)). Both carry the workflow run's id in a
   hidden marker, and it lists what is already on the pull request before it posts, so a re-run of
   the job posts neither twice.

## Agents and models {#agents-and-models}

Each expert is one pi process in the review container, run as the unprivileged user `agent`, with
read-only tools over the checkout:

```sh
pi --mode json --no-session \
  --tools read,grep,find,ls \
  --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve
```

The image carries pi's `settings.json` and `models.json` from `cmd/eyeful-action/pi/`: the model is
DeepSeek V4.1 Flash on Volcengine Ark. pi calls
`http://127.0.0.1:8081`, where `eyeful-action` forwards `POST /chat/completions`, and nothing else,
to that provider with the key `EYEFUL_MODEL_KEY`. pi holds no key. When the
provider answers 429 or a server error, pi waits and resumes the same turn, up to eight times, 5 s
doubling to at most 60 s each; eyeful reads only pi's last answer, so a run that recovered counts.

[Jev](../ARCHITECTURE.md#jev) is called on TypeSafe's API with `EYEFUL_TYPESAFE_API_KEY`.

## The sandbox {#the-sandbox}

Each review gets its own sandbox container, removed when the job ends. It runs setup once and each
test on a fresh copy, as `eyeful review` does.

- It holds no credential. It mounts the review's directory read-only and clones the two commits from
  the review container's checkout.
- The commands come from the base commit, so a pull request cannot change what runs.
- For now it can reach the whole internet, not only package registries.

## Credentials {#credentials}

| Credential         | Held by                                   | Used for                          |
| ------------------ | ----------------------------------------- | --------------------------------- |
| `GITHUB_TOKEN`     | `eyeful-action`, in the review container  | Cloning, reading Actions, posting |
| Model provider key | `eyeful-action`, from a repository secret | Forwarding pi's model calls       |
| TypeSafe API key   | `eyeful-action`, from a repository secret | Jev                               |

The workflow runs on `pull_request_target` and `issue_comment`, which see the repository's secrets
even for a pull request from a fork. It checks out only the base branch's copy of eyeful, so the
pull request cannot change the action that runs; the pull request's code runs only in the sandbox.

`eyeful-action` removes every `EYEFUL_` variable from its own environment once it has read its
configuration, so git and pi do not inherit the tokens. The container's start environment, which
`/proc/1/environ` shows, still holds them, so pi runs as the user `agent` through `setpriv`. The
kernel lets only the root user read that file and `eyeful-action`'s memory; `agent` keeps the
`dac_override` capability, so it can still read the checkout and the files eyeful writes for it.
