# Configuration reference

eyeful finds how to build and test the project under review itself. The local settings file holds
the user's choices for `eyeful review`. The server is configured with environment variables and
flags.

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

## Server {#server}

Every flag has a matching environment variable, and `eyeful-server serve --help` lists both. Secrets and
other variables are kept in separate places, both at the repository root; no app or package has its
own configuration file. When a required value is missing, the tool fails instead of guessing.

### Secrets: `.env` {#secrets}

`.env` is ignored by git. mise loads it with `redact`, so the values do not appear in task output.
`.env.example` lists exactly these.

| Secret                     | Purpose                                                                                         |
| -------------------------- | ----------------------------------------------------------------------------------------------- |
| `EYEFUL_DATABASE_URL`      | PostgreSQL URL. Required.                                                                       |
| `EYEFUL_TEST_DATABASE_URL` | A database that `go test` may wipe. If unset, those tests are skipped.                          |
| `EYEFUL_TYPESAFE_API_KEY`  | TypeSafe API key, for [Jev](ARCHITECTURE.md#jev). Optional: without it findings are not merged. |

TODO(source): the GitHub App's ID, private key and webhook secret join this table with the webhook.

### Variables: `mise.toml` and flags {#variables}

Everything that is not a secret. `DOCS_URL` is set in the `[env]` section of `mise.toml`; on a
server the rest come from the platform's environment.

| Variable           | Purpose                                                                                       |
| ------------------ | --------------------------------------------------------------------------------------------- |
| `EYEFUL_ADDR`      | Listen address. Default `:8080`.                                                              |
| `EYEFUL_LOG_LEVEL` | `debug`, `info`, `warn` or `error`. Default `info`.                                           |
| `DOCS_URL`         | Public URL of the docs site, used for the sitemap, robots and `llms.txt`. Set in `mise.toml`. |

Some settings are flags only: `--workers` (2), `--lease-ttl` (`30s`), `--poll` (`1s`), `--max-attempts`
(3), `--shutdown-timeout` (`30s`) and `--no-migrate`. The server refuses to start unless the
workers, the lease TTL and the poll interval are positive. TODO(config): move to a
`conf/eyeful.example.toml` once configuration becomes nested (dispatch tiers, sandboxes).
