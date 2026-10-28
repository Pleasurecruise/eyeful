# eyeful

<p align="center"><img src="docs/public/brand/eyeful-rounded.png" alt="eyeful sea otter logo" width="160"></p>

<p align="center">
  <a href="https://github.com/Pleasurecruise/eyeful/actions/workflows/ci.yml"><img src="https://github.com/Pleasurecruise/eyeful/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.yiming1234.cn/eyeful/"><img src="https://img.shields.io/badge/docs-eyeful-0a7ea4" alt="Docs"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue" alt="License: AGPL-3.0"></a>
  <img src="https://img.shields.io/badge/status-in%20development-orange" alt="Status: in development">
  <a href="https://github.com/Pleasurecruise/eyeful/stargazers"><img src="https://img.shields.io/github/stars/Pleasurecruise/eyeful?style=flat" alt="GitHub stars"></a>
</p>

> "Given enough eyeballs, all bugs are shallow." — Linus's Law

eyeful reviews code changes with a group of agents and tries to prove each bug it reports with a
test it runs itself. A finding counts as verified only when eyeful's verifier, not the agent that
wrote it, saw its reproduction fail on the change and not on the base commit; the suggested fix is
then checked against the same test. Everything else is marked with the weaker evidence it has. eyeful never approves a change; a person does.

It runs on your machine with the coding agent you already use (Claude Code, Codex or pi), from the
command line or a desktop app. eyeful is in active development.

## How a review works

<p align="center"><img src="docs/public/brand/harness.svg" alt="How an eyeful review works: prepare, plan, review, merge, verify, report" width="100%"></p>

1. **Prepare.** eyeful finds the project's commands from its manifests and you confirm them. The
   lint command is the CI check; the test suite runs on the change, and on the base where it fails.
2. **Plan.** Rules set aside lockfiles, generated and vendored files. The rest is grouped by intent
   with [pulls.review](https://github.com/antfu/pulls.review), and Go picks the experts each group
   needs.
3. **Review.** Up to seven experts (design, correctness, skeptic, security, tests, usability,
   readability) read their groups in parallel with read-only tools, each against the published
   standards for its area.
4. **Merge.** Findings on the same lines that describe one problem are merged with
   [Jev](https://docs.typesafe.ai), and the comment records how many experts reported it.
5. **Verify.** For each `error` or `warning` finding that a test could show, an agent writes a
   reproduction and the verifier runs it five times on the change, once on the base, then with the
   suggested fix. What cannot run is read by a judge, which may dispute it.
6. **Report.** Comments are labelled `issue`, `question`, `suggestion` or `nitpick` and ranked:
   undisputed before disputed, reproduced before unreproduced, then by how many experts agree.

Details: [A review, end to end](docs/REVIEW.md).

## Quick start

You need git, [mise](https://mise.jdx.dev), and one of Claude Code, Codex or pi signed in. There is
no release yet, so eyeful is built from source:

```sh
git clone https://github.com/Pleasurecruise/eyeful && cd eyeful
mise trust && mise install && mise run setup
mise run install          # puts eyeful in ~/.local/bin; add it to PATH
eyeful connect claude     # or codex, pi

cd ~/your-project
eyeful review             # review your uncommitted changes
```

`eyeful review --head HEAD --base main` reviews a branch instead. Each run is saved under
`.eyeful/runs/` as Markdown, findings JSON and SARIF. `mise run dev:desktop` opens the same review in
a window, with the findings beside the diff.

### Reproductions

eyeful finds how to install and test the project from its `go.mod`, `Cargo.toml`, `pyproject.toml`
or `package.json` ([Project commands](docs/CONFIGURATION.md#project)), with rules and no model. Before
the review starts it shows those commands and runs them only after you confirm, in a throwaway
checkout of the snapshot. The agents themselves get read-only tools.

When it finds no test command, nothing runs and a judge reads every finding. `--test` and
`--test-one` give the commands yourself; `--verification read_only` never runs anything.

## Status and limits

- **Works:** local reviews from the CLI and the desktop app.
- **In progress:** the evaluation on SWE-bench Verified, Code Review Bench and ReviewBench
  ([Evaluation](docs/EVALUATION.md)). How eyeful compares with other reviewers is not yet known.
- **In progress:** reviews of GitHub pull requests through a GitHub Action, verifying findings in a
  sandbox container per review ([Running in the cloud](docs/cloud/CLOUD.md)).
- It can only prove what a unit-sized test shows. Timing, performance and behaviour that needs a
  database or a browser are read by the judge, never run.
- It reports only what the change introduces, and a review without findings proves nothing.
- Locally there is no sandbox: confirmed commands run with your permissions.

## Documentation

<https://github.yiming1234.cn/eyeful/>, starting with [Running locally](docs/LOCAL.md). The
[roadmap](docs/ROADMAP.md) says what each version delivers.

## License

[AGPL-3.0-only](LICENSE).
