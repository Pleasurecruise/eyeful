# Configuration reference

There are three kinds of configuration. `.eyeful/config.yml`, in the repository being reviewed, tells a
review how to build and test the project. The local settings file holds the user's choices for
`eyeful review`. The server is configured with environment variables and flags.

## Project {#project}

`.eyeful/config.yml` lives in the repository under review; eyeful keeps its own
[`.eyeful/config.yml`](../.eyeful/config.yml) for reviewing itself. `eyeful review` reads it, and an
unknown key is an error. The cloud does not read it yet.

```yaml
setup: pip install -e ".[test]"
lint: ruff check .
test: pytest -x -q --junitxml=report.xml
test_one: pytest -x -q {test}
coverage: pytest --cov --cov-report=json
e2e: npx playwright test
risk: ['auth/**', 'payments/**', 'migrations/**']
skip: ['docs/api/**']
```

| Key        | Used by                                                                                                 |
| ---------- | ------------------------------------------------------------------------------------------------------- |
| `setup`    | Preparation: installs dependencies. In the cloud it is the only step that can reach package registries  |
| `lint`     | The local CI check, and the readability expert's `lint` tool                                            |
| `test`     | The verifier, for the suite: before the review, to find the tests the change breaks, and after each fix |
| `test_one` | The verifier, for one reproduction test; `{test}` is replaced by the test's identifier, quoted          |
| `coverage` | The tests expert's `coverage_diff` tool                                                                 |
| `e2e`      | The usability expert's `e2e_run` tool                                                                   |
| `risk`     | The plan: a group that touches these paths is a core group                                              |
| `skip`     | Triage: these paths go to no expert and are listed as not reviewed                                      |

These commands are the only project code a review runs, and the user confirms them before the
review starts ([Before it starts](REVIEW.md#before-it-starts)). Without `test` and `test_one`,
findings stay unverified.

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
