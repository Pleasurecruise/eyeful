# Deployment

This page covers what eyeful ships, where each part runs, and how to turn on cloud reviews. Running
eyeful from a checkout is in [Development](../DEVELOPMENT.md), every variable is in
[Configuration](../CONFIGURATION.md#action), and release timing is in the [Roadmap](../ROADMAP.md).

Cloud reviews run in GitHub Actions through the composite action `action.yml` at the repository
root. The local side is the desktop app and the `eyeful` command, and needs no server. Nothing is
published until CI passes on it: the documentation site goes to GitHub Pages after each green CI run
on `main` (`docs.yml`), and a `v*` tag runs CI and then GoReleaser (`release.yml`, `.goreleaser.yml`),
which attaches the `eyeful` CLI for Linux and macOS, as archives, `.deb` and `.rpm`, to a GitHub
release. No version has been tagged yet. CI (`.github/workflows/ci.yml`) runs `mise run check` and builds the review and sandbox images.

## Artifacts

| Artifact                  | Built by                                                      | Runs on                    |
| ------------------------- | ------------------------------------------------------------- | -------------------------- |
| Review and sandbox images | `action.yml` in each job (`Dockerfile`, `Dockerfile.sandbox`) | A GitHub Actions runner    |
| Desktop app               | `mise run package:desktop` (Wails)                            | macOS, Windows, Linux      |
| `eyeful`                  | `mise run install`, or a `v*` tag (GoReleaser)                | The user's machine         |
| Documentation site        | `mise run build:docs` (VitePress)                             | GitHub Pages at `DOCS_URL` |

The review image holds `eyeful-action`, git, pi `1.4.0` under `/opt/pi`, the user `agent` with pi's
settings from `cmd/eyeful-action/pi/`, and `/usr/local/bin/pi`, which runs pi as `agent`. The sandbox image holds `eyeful-sandbox` and [mise](https://mise.jdx.dev), configured by
`cmd/eyeful-sandbox/mise.toml`. It preinstalls Node, Go, Python, Rust, pnpm, Yarn, Bun, uv and
Poetry, and a project's own version files (`mise.toml`, `.nvmrc`, `packageManager` in `package.json`,
`go.mod`, `.python-version`, `rust-toolchain.toml`) select the version that runs; mise installs a
missing one on first use.

## Cloud

```mermaid
flowchart TB
    gh["Pull request<br/>opened, or mentioned"] --> job["Actions job<br/>review.yml"]
    job --> action["Review container<br/>eyeful-action"]
    action -->|"127.0.0.1:8081"| proxy["models proxy"] --> provider["Model provider"]
    action -->|Jev| typesafe["TypeSafe API"]
    action -->|clone, publish| github["GitHub"]
    action -->|"eyeful-sandbox:8080"| sandbox["Sandbox container<br/>no credentials"]
    sandbox -->|"reads /work, read-only"| action
```

Both containers share a Docker network `eyeful` and the directory `$RUNNER_TEMP/eyeful`, mounted at
`/work`: read-write for the review, read-only for the sandbox.

### Turn on reviews {#turn-on-reviews}

You need a Volcengine Ark API key with the Coding Plan ([console](https://console.volcengine.com/ark)),
a [TypeSafe](https://docs.typesafe.ai) API key, and the [GitHub CLI](https://cli.github.com) signed
in. In the repository, on its default branch, run:

```sh
gh secret set EYEFUL_MODEL_KEY          # paste the Volcengine Ark key
gh secret set EYEFUL_TYPESAFE_API_KEY   # paste the TypeSafe key
curl -fsSL https://raw.githubusercontent.com/Pleasurecruise/eyeful/main/.github/workflows/eyeful.yml \
  --create-dirs -o .github/workflows/eyeful.yml
git add .github && git commit -m "ci: review pull requests with eyeful" && git push
```

The first two commands store the keys as repository secrets. The third copies eyeful's own
`.github/workflows/eyeful.yml`, which calls the reusable workflow `.github/workflows/review.yml`.
That workflow holds the rest: who may start a review, one review per pull request at a time, and the
action's inputs. It never checks out the pull request; `eyeful-action` clones it and runs its code
only in the sandbox.

## Local

```mermaid
flowchart LR
    user(["User"]) --> client["Desktop or<br/>eyeful review"]
    client --> repo["git repository<br/>branches, worktrees, changes"]
    client -->|CLI| agents["Claude Code, Codex, pi<br/>user's own sign-in"]
    client --> worktree["A disposable checkout of the snapshot<br/>confirmed commands only"]
```

Local mode installs only the app itself. It needs no server, database, eyeful account or model key,
because the agents are the coding tools the user is already signed in to; only Jev needs a TypeSafe
API key.

`eyeful review` runs a review with the agent chosen by `eyeful connect`; the desktop app does the
same from a window ([The desktop app](../LOCAL.md#desktop)). What
a local review may run, and how it is contained, is described in [Running locally](../LOCAL.md).
