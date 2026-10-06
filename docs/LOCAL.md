# Running locally

A local review runs entirely on the user's machine, from the `eyeful review` command or the
[desktop app](#desktop). It needs no eyeful account, server or model key, because the agents are the
coding tools the user is already signed in to.

## Commands

`mise run install` puts `eyeful` on the `PATH` ([Local development](DEVELOPMENT.md#commands)).

```sh
eyeful provider                             # the agents, which is connected, which is installed
eyeful connect claude                       # or codex, pi
eyeful review                               # the planner picks the experts
eyeful review security                      # one expert reviews every changed file
eyeful review correctness security --base main
eyeful commit                               # the agent writes the message; you confirm it
```

`provider` shows each agent eyeful can drive, whether it is the connected one, and whether its
command-line tool is on the `PATH`. `connect` checks that the tool is installed, reads its version,
then saves it. There is no setting for how much a
review does: eyeful chooses the level for each change and starts experts only where the change calls
for them ([Levels](LEVELS.md#choosing-the-level)).

Naming experts skips the planner: those experts review every file triage kept; with no name the
planner chooses. `review` takes only `--base` and `--head` (what to review), `--verification`
(`read_only`, the default, `execution` or `none`) and `--yes` (confirm the project commands). The
token budget (3,000,000) and the limit per project command (10 minutes) are fixed. Where the
settings are kept is in [Configuration](CONFIGURATION.md#local).

`commit` commits every change in the working copy, staged or not, as `git commit` would, hooks
included. The connected agent writes the message from the diff with the `git-commit` skill
([Experts](EXPERTS.md#skills-and-tools)) and only reads; eyeful makes the commit after you confirm
the message, or with `--yes`, and `-m` gives the message yourself. It never pushes.

A review writes nothing to your working copy except eyeful's own `.eyeful/runs/`, which ignores
itself in git. Project commands, which `execution` needs, run only in a
checkout of the [snapshot](#snapshot) that eyeful creates and removes itself.

```sh
eyeful review correctness                                   # uncommitted changes; a judge agent checks each finding
eyeful review --verification execution                      # the same, with each reproduction run
eyeful review --head HEAD --base main --verification execution
```

## The desktop app {#desktop}

The desktop app does what the commands do, through the same `local/app` calls, in a window:

| In the window                                              | Same as                                      |
| ---------------------------------------------------------- | -------------------------------------------- |
| The agent menu: each agent, the connected one, the missing | `eyeful provider`; choosing one is `connect` |
| The branch menu: the current branch, other worktrees       | opening that worktree's directory            |
| Compare with: uncommitted changes only, or a branch        | no flag, or `--base`                         |
| A custom range: base and head                              | `--base`, `--head`                           |
| Experts, or the planner's choice                           | the expert names                             |
| Verification: judge agent, run tests, none                 | `--verification`                             |
| A dialog listing the project commands, Run or Cancel       | the `[y/N]` prompt; there is no `--yes`      |
| Findings beside the diff and on their lines; the log tab   | the printed report and the terminal output   |
| Review or Commit under the file list; the generate button  | `eyeful review`; `eyeful commit`, `-m`       |

It opens on the repository it was started in; started outside one, as from the Finder, it opens the
last repository it showed. The folder button opens another. Because an app started from the Finder or
a desktop menu does not get the terminal's `PATH`, it reads `PATH` from the user's login shell at
start, so it finds the same agent CLIs as `eyeful provider`. By default it
shows the uncommitted changes of the current branch; choosing a branch that another worktree has
checked out opens that worktree, and a branch that is not checked out anywhere can only be compared
against. The changed files are a flat list, as in GitHub Desktop, so a deep path never hides a file name. Commit is available only while the subject is the uncommitted changes. The file list
and the review panel beside the diff can be resized and collapsed; the review panel opens when a
review starts. The diff is the
subject's, read the same way as the review reads it. Cancelling stops the review the way Ctrl-C
does. The run directory and its files are the same as the command's.

## What can be reviewed {#what-can-be-reviewed}

`eyeful review` runs in the repository that contains the current directory, found through its `.git`, and stops when there is none.

| Subject                                     | Flags                  | Base                                     | Head                                 |
| ------------------------------------------- | ---------------------- | ---------------------------------------- | ------------------------------------ |
| Uncommitted changes                         | none                   | `HEAD`, or nothing in a new repository   | Staged, unstaged and untracked files |
| A branch together with its uncommitted work | `--base main`          | The merge base of `main` and `HEAD`      | Staged, unstaged and untracked files |
| A commit or branch                          | `--head X`, `--base Y` | The merge base of `Y` and `X`, or `HEAD` | `X`                                  |

The agents read the code in the repository, so with `--head` the head must be the checked-out
commit and the working copy clean; otherwise eyeful stops and says which branch to check out.
Local mode never reviews a pull request, does not call the eyeful server, does not use the keys
configured on it, and sends it no code, records or credentials. Pull requests are reviewed in the
cloud ([Running in the cloud](CLOUD.md)).

## The snapshot {#snapshot}

Before anything runs, eyeful fixes the subject as a snapshot: the base commit and one commit that
holds the reviewed code. With `--head` that commit is the head. For uncommitted changes eyeful makes
one from a copy of the index, the way pulls.review reads a working tree: it adds every file that is
not ignored, writes the tree and commits it with a fixed author and date, on top of `HEAD`. The commit
goes into a temporary object directory described below, never into the repository, and no branch, ref or index
points to it. Because the author and date are fixed, the same changes always give the
same commit.

The run directory keeps the snapshot as `snapshot.json` (repository, base, head and commit) and the
diff as `change.diff`, beside the findings, the way revmux keeps each round's input with its output.

The snapshot's git objects never enter the repository. eyeful writes them to a temporary object
directory that borrows the repository's own objects through git's `objects/info/alternates`, runs
every git command and project command of the review against it, and deletes it when the review
ends. Afterwards the repository's object store is exactly as it was.

When the review ends, eyeful takes the snapshot again. If it differs, or with `--head` the head is no
longer checked out cleanly, eyeful says so: the agents read the working copy, so they may have seen
newer code. The findings are kept and refer to the recorded snapshot. Project commands are not
affected, because they run in the snapshot's own checkout.

## How a local review runs {#how-a-local-review-runs}

1. eyeful finds the repository, takes the snapshot and checks that the code the agents will read is
   the code under review. The run's output goes to `.eyeful/runs/`, which holds a `.gitignore` of `*`,
   so git never sees it and the next review's snapshot leaves it out.
2. It reads [`.eyeful/config.yml`](CONFIGURATION.md#project) from the repository, which step 1
   showed holds the snapshot, checks the request (experts, level, budget, globs) before any agent
   starts, and prints the subject, the snapshot, experts, level, agent and verification. With
   `execution` it checks out the snapshot with `git worktree add --detach` into the system's
   temporary directory, with hooks turned off, and prints the commands it will run there, and the user confirms them, unless
   exactly the same commands were confirmed before in this repository. `--yes` confirms without
   asking; without a terminal or `--yes`, the review stops. eyeful resets the checkout after every
   run and removes it when the review ends. The commands never run anywhere else.
3. With `execution`, the CI check runs `setup`, then `lint` in the checkout. If `lint` fails, the CI
   agent diagnoses the failure and the review ends ([Planning and triage](PLANNER.md#ci-check)).
   With `read_only` or `none` no command runs, so there is no CI check.
4. Triage sorts the files, and the planner makes a plan from them and the messages of the commits
   in the range (the newest 50), the same as in the cloud.
5. Each expert, the planner, the fix agent and the summarizer is a new session with the connected
   agent's command-line tool ([Agents](#agents)).
6. With `execution`, the verifier runs each reproduction with `test_one` and the suite with `test`
   in the checkout. With `read_only`, a judge agent reads the code and decides whether each finding
   is real.
7. `eyeful review` prints the findings in the form of Claude Code's ultrareview, important ones
   first ([In the terminal](REPORT.md#terminal)). It writes `snapshot.json`, `change.diff`,
   `review.md`, `findings.json`, `results.sarif`, `result.json` and
   each stage's checkpoint to `.eyeful/runs/<time>/`.

## Agents {#agents}

eyeful runs the connected agent's own command-line tool in its non-interactive mode, one new process
per call, in the repository root, the way revmux and pulls.review drive them. The prompt, composed by
`workflow/prompts`, goes in on standard input, so it has no length limit. It carries what the role
needs: the diffs (for the planner, those of the files triage kept in its scope), and for an expert
the name, description and path of each skill offered to it, written into the run directory for the
length of the review and removed after it. The agent reads the skills that apply, and other files with its own read-only tools and
ends with one JSON block that matches the role's schema, which is the result the role would
otherwise submit ([Prompt, skills and tools](EXPERTS.md#skills-and-tools)). If the block is missing
or does not match, eyeful runs the call once more with the reason added, then counts it as failed.
No MCP server is configured for the agent: MCP is served only by the cloud ([API](API.md#mcp-planned)).
Because the agent cannot run commands, the only commands that run are the confirmed project
commands, in the snapshot's checkout; the agent hands in a reproduction and the verifier runs it.

| Agent  | Run as                                                                                                                  | Read-only through           | Result                                                         | Model                                       |
| ------ | ----------------------------------------------------------------------------------------------------------------------- | --------------------------- | -------------------------------------------------------------- | ------------------------------------------- |
| claude | `claude -p --output-format json --json-schema … --no-session-persistence --strict-mcp-config --setting-sources project` | `--tools Read,Grep,Glob`    | `structured_output`, checked by Claude Code against the schema | an expert's tier: `haiku`, `sonnet`, `opus` |
| codex  | `codex exec --json --ephemeral --skip-git-repo-check -`                                                                 | `--sandbox read-only`       | the last `agent_message`                                       | Codex's default                             |
| pi     | `pi --mode json --no-session`                                                                                           | `--tools read,grep,find,ls` | the last assistant message                                     | pi's default                                |

Each tool uses the sign-in it already has. eyeful removes `ANTHROPIC_API_KEY` from Claude Code's
environment, as revmux does, so a key set for other programs does not turn a review into billed API
use, and `--setting-sources project` keeps the user's own Claude Code settings, hooks and MCP servers
out of the review. `connect`, and every review before it starts, checks that the tool is on the
`PATH` and reads its version. Whether it is signed in shows on the first call, and the error then
says how to sign in, such as running `claude` and `/login`.

Tokens, and the cost where the tool reports it (Claude Code and pi), come from each call's output
and count against the review's budget. Usage is billed to the user's own subscription or key for
that tool, like any other use of it.

## Local risks {#local-risks}

The user picks the code to review, but eyeful still guards against it:

| Risk                                                    | Guard                                                                                                               |
| ------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| `.eyeful/config.yml` names a harmful command            | Commands run only after the user confirms them, and again after any change                                          |
| A repository hook runs while eyeful prepares the change | eyeful's git commands run with hooks turned off                                                                     |
| Content in the code steers an agent                     | The agent's CLI runs with read-only tools only (`--tools`, `--sandbox read-only`)                                   |
| An agent's test identifier or edit is crafted           | The identifier is passed to `test_one` as one quoted word; edits must stay inside the repository and outside `.git` |
| A project command does damage                           | It runs only in eyeful's checkout of the snapshot, never in your working copy                                       |
| The repository ships its own confirmation               | Confirmations are kept in the user's configuration directory, under the repository's path, never in the repository  |
| A command that times out leaves processes running       | eyeful kills the command's whole process group, not only its shell                                                  |
| The working copy changes during the review              | The snapshot is fixed first, and eyeful reports when the agents may have read newer code                            |
| The branch is someone else's, such as a checked-out PR  | Planned: eyeful warns and suggests a cloud review                                                                   |

A confirmed command runs with the user's own permissions, as it would if the user ran it, so there is
no further sandbox on the user's machine. Code from someone else belongs in the cloud sandbox
([Running in the cloud](CLOUD.md#the-sandbox)).
