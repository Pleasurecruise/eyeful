# Running locally

A local review runs on the user's machine, from `eyeful review` or the [desktop app](#desktop). It
needs no eyeful account, server or model key: the agents are the coding tools the user is already
signed in to.

## Commands

`mise run install` puts `eyeful` on the `PATH` ([Local development](DEVELOPMENT.md#commands)).

```sh
eyeful provider                             # the agents: connected, installed
eyeful connect claude                       # or codex, pi
eyeful review                               # uncommitted changes; the planner picks the experts
eyeful review security                      # one expert reviews every changed file
eyeful review --head HEAD --base main --verification execution
eyeful commit                               # the agent writes the message; you confirm it
```

`review` takes:

| Flag               | Meaning                                                                                                                                                     |
| ------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| expert names       | Skip the planner; these experts review every file triage kept                                                                                               |
| `--base`, `--head` | What to review ([below](#what-can-be-reviewed))                                                                                                             |
| `--verification`   | `read_only` (default), `execution` or `none`                                                                                                                |
| `--yes`            | Confirm the project commands without asking                                                                                                                 |
| `--run`            | The run directory. A review restarted with the same one skips the stages saved there and runs again the steps whose agent call failed or the budget refused |
| `--description`    | A file with the author's description, such as a pull request's title and body                                                                               |

The token budget (3,000,000) and the limit per project command (10 minutes) are fixed. Settings are
in [Configuration](CONFIGURATION.md#local).

`commit` commits every change in the working copy, hooks included. The connected agent writes the
message with the `git-commit` skill; `-m` gives it yourself. It never pushes.

A review writes nothing to the working copy except `.eyeful/runs/`, which ignores itself in git.

## The desktop app {#desktop}

The desktop app does what the commands do, through the same `local/app` calls:

| In the window                                        | Same as                           |
| ---------------------------------------------------- | --------------------------------- |
| The agent menu                                       | `eyeful provider`, `connect`      |
| The branch menu: the current branch, other worktrees | Opening that worktree's directory |
| Compare with: uncommitted changes, or a branch       | No flag, or `--base`              |
| A custom range                                       | `--base`, `--head`                |
| Experts, or the planner's choice                     | The expert names                  |
| Verification: judge agent, run tests, none           | `--verification`                  |
| A dialog listing the project commands                | The `[y/N]` prompt                |
| Findings beside the diff; the log tab                | The printed report                |
| Review or Commit under the file list                 | `eyeful review`, `eyeful commit`  |

It opens on the repository it was started in, or the last one it showed. It reads `PATH` from the
user's login shell, so it finds the same agent CLIs as the terminal. The diff is read again whenever
the window regains focus.

## What can be reviewed {#what-can-be-reviewed}

`eyeful review` runs in the repository that contains the current directory.

| Subject                            | Flags                  | Base                                | Head                                 |
| ---------------------------------- | ---------------------- | ----------------------------------- | ------------------------------------ |
| Uncommitted changes                | none                   | `HEAD`                              | Staged, unstaged and untracked files |
| A branch with its uncommitted work | `--base main`          | The merge base of `main` and `HEAD` | Staged, unstaged and untracked files |
| A commit or branch                 | `--head X`, `--base Y` | The merge base of `Y` and `X`       | `X`                                  |

With `--head` the head must be checked out and the working copy clean, because the agents read the
code in the repository. Local mode never reviews a pull request and never calls the eyeful server
([Running in the cloud](cloud/CLOUD.md)).

## The snapshot {#snapshot}

Before anything runs, eyeful fixes the subject as a snapshot: the base commit and one commit holding
the reviewed code. For uncommitted changes it builds that commit from a copy of the index, with a
fixed author and date, so the same changes always give the same commit.

- The commit goes into a temporary object directory that borrows the repository's objects through
  `objects/info/alternates`. Nothing is written to the repository, and the directory is deleted at
  the end.
- The run directory keeps `snapshot.json` and `change.diff`.
- When the review ends, eyeful takes the snapshot again and says so if it differs: the agents may
  have read newer code.

## How a local review runs {#how-a-local-review-runs}

1. eyeful finds the repository and takes the snapshot.
2. It reads [`.eyeful/config.yml`](CONFIGURATION.md#project), checks the request and prints the
   subject, experts, agent and verification.
3. With `execution` it checks the snapshot out with `git worktree add --detach` into the temporary
   directory, hooks off, and prints the commands it will run there. The user confirms them unless
   the same commands were confirmed before in this repository. The checkout is reset after every
   run and removed at the end.
4. With `execution` the CI check runs `setup`, then `lint`
   ([Planning and triage](PLANNER.md#ci-check)).
5. Triage sorts the files, and the planner plans from them and the newest 50 commit messages.
6. With `execution` the verifier runs `test`, and the last review's reproductions
   ([Regressions](VERIFICATION.md#regressions), [Rounds](REPORT.md#rounds)).
7. The experts review. Reproductions are written and run, and the judge reads the rest
   ([Evidence and verification](VERIFICATION.md)).
8. The findings are printed ([In the terminal](REPORT.md#terminal)) and saved under
   `.eyeful/runs/<time>/`: `snapshot.json`, `change.diff`, `review.md`, `findings.json`,
   `results.sarif`, `result.json`, each stage's checkpoint, and under `agents/` the prompt and raw
   output of every agent call.

## Agents {#agents}

eyeful runs the connected agent's command-line tool in non-interactive mode, one new process per
call, in the repository root. The prompt goes in on standard input. The agent ends with one JSON
block matching the role's schema; a missing or invalid block is retried once.

| Agent  | Run as                                                                                                                                                                                                     | Read-only through           | Model                               |
| ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- | ----------------------------------- |
| claude | `claude -p --output-format stream-json --verbose --include-partial-messages --json-schema … --no-session-persistence --strict-mcp-config --setting-sources project --disable-slash-commands`               | `--tools Read,Grep,Glob`    | the tier: `haiku`, `sonnet`, `opus` |
| codex  | `codex exec --json --ephemeral --skip-git-repo-check --ignore-user-config --ignore-rules --disable hooks --disable plugins --disable apps --enable skip_host_skill_discovery -c project_doc_max_bytes=0 -` | `--sandbox read-only`       | Codex's default                     |
| pi     | `pi --mode json --no-session --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve`                                                                                   | `--tools read,grep,find,ls` | pi's default                        |

- **Isolation.** Each tool runs without the user's settings, hooks, MCP servers, extensions, skills
  and `AGENTS.md` files. eyeful removes `ANTHROPIC_API_KEY` from Claude Code's environment so a
  review is not billed as API use.
- **Sign-in.** Each tool uses the sign-in it has. `connect` and every review check that the tool is
  on the `PATH`.
- **Tokens.** eyeful counts what a call processes for the first time: input, output and cache
  writes. Cache reads are left out. Usage is billed to the user's own subscription.
- **Time.** A call that writes nothing for 2 minutes, or runs for 20, is stopped. A call stopped for
  silence or a tool error runs once more.
- **Concurrency.** At most four experts run at once.

No MCP server is configured for the agent.

## The classifier {#classifier}

`local/classify` asks [Jev](ARCHITECTURE.md#jev) through [go-jev](https://github.com/mattn/go-jev),
with the key in [`EYEFUL_TYPESAFE_API_KEY`](CONFIGURATION.md#secrets). A request carries each finding's file, line, subject and
discussion, and no code.

## Local risks {#local-risks}

| Risk                                              | Guard                                                                                                                                                           |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `.eyeful/config.yml` names a harmful command      | Commands run only after the user confirms them, and again after any change                                                                                      |
| A repository hook runs while eyeful prepares      | eyeful's git commands run with hooks off                                                                                                                        |
| Content in the code steers an agent               | The agent's CLI has read-only tools only                                                                                                                        |
| With `execution`, a steered agent writes the test | Not guarded: a reproduction test and a fix are code an agent wrote, and they run with the user's permissions and environment. Use `execution` on code you trust |
| An agent's test identifier or edit is crafted     | The identifier is passed as one quoted word; edits stay inside the repository and outside `.git`; a new file never replaces an existing one                     |
| A project command does damage                     | It runs only in eyeful's checkout of the snapshot                                                                                                               |
| The repository ships its own confirmation         | Confirmations are kept in the user's configuration directory                                                                                                    |
| A timed-out command leaves processes running      | eyeful kills the whole process group                                                                                                                            |
| The working copy changes during the review        | The snapshot is fixed first, and eyeful reports a difference at the end                                                                                         |
| An agent reads files outside the repository       | Not guarded: the CLI's read tools reach every file the user can read                                                                                            |

A confirmed command runs with the user's own permissions. There is no sandbox on the user's
machine; code from someone else belongs in the cloud sandbox
([Running in the cloud](cloud/CLOUD.md#the-sandbox)).
