# Configuration reference

eyeful finds how to build and test the project under review itself. The local settings file holds
the user's choices for `eyeful review`. The cloud action is configured with environment variables
and flags.

## Project commands {#project}

Rules, with no model, pick the commands from the files at the root of the snapshot. The first row
that matches is used.

| Files                                                     | `setup`                                                  | `test`, `test_one`                                                                             | `lint`                                            |
| --------------------------------------------------------- | -------------------------------------------------------- | ---------------------------------------------------------------------------------------------- | ------------------------------------------------- |
| `go.mod`                                                  | `go mod download`                                        | `go test ./...`, with `-run {test}`                                                            | `golangci-lint run ./...` with a `.golangci` file |
| `Cargo.toml`                                              | `cargo fetch`                                            | `cargo test`, with `{test}`                                                                    |                                                   |
| `pyproject.toml`, `setup.py` or `setup.cfg` naming pytest | `uv sync`, `poetry install` or `pip install` by lockfile | `pytest -q`, with `{test}`, through `uv run` or `poetry run` when they install                 | `ruff check .` when ruff is configured            |
| `package.json`                                            | install with the lockfile's package manager              | `vitest run`, `jest` or `mocha` when a dependency, with `{test}`; else the `test` script alone | the `lint` script                                 |

| Command    | Used by                                                                                                 |
| ---------- | ------------------------------------------------------------------------------------------------------- |
| `setup`    | Preparation: installs dependencies                                                                      |
| `lint`     | The local CI check                                                                                      |
| `test`     | The verifier, for the suite: before the review, to find the tests the change breaks, and after each fix |
| `test_one` | The verifier, for one reproduction test; `{test}` is replaced by the test's identifier, quoted          |

A project that matches no row, or a Python project that does not name pytest, gets no commands:
nothing runs and the judge reads every finding, and the review says so before it starts.

`eyeful review --setup`, `--lint`, `--test` and `--test-one` replace what eyeful finds, and
`--no-detect` runs only the commands given. The evaluation uses them to run SWE-bench's tests in a
container.

These commands are the only project code a review runs, and the user confirms them before the
review starts ([Before it starts](REVIEW.md#before-it-starts)).

Files that the repository's `.gitattributes` marks `linguist-generated` or `linguist-vendored` go to
no expert ([Triage](PLANNER.md#triage)).

## Local settings {#local}

`eyeful connect` writes `eyeful/settings.json` in the user's configuration
directory (`~/Library/Application Support` on macOS, `$XDG_CONFIG_HOME` or `~/.config` on Linux):

```json
{
	"agent": "claude",
	"repository": "/Users/me/src/app"
}
```

`eyeful review` fails until an agent is connected. `repository` is the last repository the desktop
app showed, which it opens when started outside one. Beside the settings, `eyeful/confirmed` holds one
hash per confirmation, of the repository's path and the commands confirmed there, so a repository
cannot ship a confirmation of its own. Each project keeps its review runs under `.eyeful/runs/`.
That directory holds a `.gitignore` of `*`, so git never sees it and it never ends up in a review.

## Action {#action}

Every flag of `eyeful-action` has a matching environment variable, and `eyeful-action --help` lists
both. `action.yml` sets them from its inputs and the job's environment. When a required value is
missing, the program fails instead of guessing.

### Secrets {#secrets}

| Secret                    | Purpose                                                                     |
| ------------------------- | --------------------------------------------------------------------------- |
| `EYEFUL_MODEL_KEY`        | API key for the provider at `EYEFUL_MODEL_URL`.                             |
| `EYEFUL_TYPESAFE_API_KEY` | TypeSafe API key for Jev. Optional; without it findings are not merged.     |
| `EYEFUL_GITHUB_TOKEN`     | The job's `GITHUB_TOKEN`, which clones, reads Actions and posts the review. |

In GitHub the first two are repository secrets passed to the action's inputs
([Deployment](cloud/DEPLOYMENT.md#turn-on-reviews)). Locally they live in the root `.env`, which git
ignores and mise loads with `redact`; `.env.example` lists them. `eyeful review` uses
`EYEFUL_TYPESAFE_API_KEY` for [Jev](ARCHITECTURE.md#jev); without it it does not merge findings.

### Variables {#variables}

Everything that is not a secret. `DOCS_URL` is set in the `[env]` section of `mise.toml`.

| Variable             | Purpose                                                                                       |
| -------------------- | --------------------------------------------------------------------------------------------- |
| `EYEFUL_PULL`        | The pull request to review.                                                                   |
| `GITHUB_REPOSITORY`  | The repository, as `owner/name`. Actions sets it.                                             |
| `GITHUB_RUN_ID`      | The workflow run; it marks what the review posts, and the review does not wait for it.        |
| `GITHUB_API_URL`     | GitHub API root. Default `https://api.github.com`.                                            |
| `EYEFUL_DIR`         | The checkout and the review's files, shared read-only with the sandbox. Default `/work`.      |
| `EYEFUL_SANDBOX_URL` | The review's sandbox. Default `http://eyeful-sandbox:8080`.                                   |
| `EYEFUL_MODEL_URL`   | OpenAI-compatible base URL of the provider serving pi's model; a repository variable.         |
| `EYEFUL_AGENT`       | Agent CLI the review connects: `pi`, `claude` or `codex`. Default `pi`.                       |
| `EYEFUL_LOG_LEVEL`   | `debug`, `info`, `warn` or `error`. Default `info`.                                           |
| `DOCS_URL`           | Public URL of the docs site, used for the sitemap, robots and `llms.txt`. Set in `mise.toml`. |

The model a cloud review uses is set in pi's own settings in `cmd/eyeful-action/pi/`, not by a
variable ([Agents and models](cloud/CLOUD.md#agents-and-models)).
