# Experts

Experts are the subagents that do the reviewing. Each covers part of the ISO/IEC 25010 product
quality model, works from the public standards for that part, and has its own checklists (skills)
and tools. The planner decides which experts a change needs ([Planning and triage](PLANNER.md)), and
the level decides how closely they look and on which models ([Levels](LEVELS.md)). `workflow` runs
each selected expert once, over every group it was picked for, up to four at a time, and checks
their reports. All six expert files exist. Of the tools
named in each expert's section, those not in [Prompt, skills and tools](#skills-and-tools) are
planned. The standards named here are listed in [References](REFERENCES.md).

## The roster {#the-roster}

| Expert      | ISO/IEC 25010 characteristic               | Model tier |
| ----------- | ------------------------------------------ | ---------- |
| design      | Maintainability: modularity, modifiability | strong     |
| correctness | Functional suitability, reliability        | strong     |
| security    | Security                                   | strong     |
| tests       | Maintainability: testability               | mid        |
| usability   | Interaction capability                     | mid        |
| readability | Maintainability: analysability             | cheap      |

The v1.0 experiment uses only correctness and security, the two experts whose findings can be
reproduced ([Evaluation](EVALUATION.md)); the other four run in local reviews. Performance
efficiency has no expert yet ([Roadmap](ROADMAP.md)). The model
tier selects a list of models through [provider routing](SCHEDULING.md#agent-runs-over-providers).

## Prompt, skills and tools {#skills-and-tools}

An agent eyeful starts gets a prompt, a set of skills and a set of tools. The prompt says who the
agent is and gives a few lines of guidance. For an expert it is the body of its expert file; the
planner, the fix agent, the judge, the summarizer and the CI agent have theirs under
`workflow/prompts/roles/`. Standards and checklists are not part of the prompt.

A skill is a folder with a `SKILL.md`, in the Agent Skills format that
[skills.sh](https://skills.sh) distributes. eyeful does not write its own skills. It takes widely
installed ones from skills.sh and vendors them unchanged under `workflow/prompts/skills/`, with the
skills CLI's `skills-lock.json`, and they are embedded in the binary. eyeful refuses to start when a
skill's files no longer match the hash in the lock file, and `mise run skills` reinstalls them from
it. An expert file names its skills; a language skill also names the files it applies to and is
offered only when the expert's group contains one.

| Skill                                            | From                                | Installs on skills.sh | Expert                       |
| ------------------------------------------------ | ----------------------------------- | --------------------- | ---------------------------- |
| `code-review`                                    | `anthropics/knowledge-work-plugins` | 9.7K                  | correctness                  |
| `tdd`                                            | `mattpocock/skills`                 | 1M                    | correctness                  |
| `golang-error-handling`, `golang-concurrency`    | `samber/cc-skills-golang`           | about 42K each        | correctness, Go files        |
| `python-error-handling`, `async-python-patterns` | `wshobson/agents`                   | 13.3K, 17.1K          | correctness, Python files    |
| `typescript-advanced-types`                      | `wshobson/agents`                   | 83.9K                 | correctness, JS and TS files |
| `security-review`                                | `getsentry/skills`                  | 18.9K                 | security                     |
| `golang-security`                                | `samber/cc-skills-golang`           | 42.4K                 | security, Go files           |
| `git-commit`                                     | `github/awesome-copilot`            | 45.9K                 | `eyeful commit` only         |

The install counts are from October 2026.

`workflow/prompts/tools.yaml` defines every tool, and each role file names the tools its role gets
and the one it submits with. The cloud runtime provides exactly those. A local review has no tool
server: the diffs are part of the input, and eyeful writes the skills into the run directory for
the length of the review, so the input lists each offered skill's name, description and path; the
agent reads the ones that apply, and any other file, with its own tools. eyeful cannot tell which skills a local expert read, so a local
review lists every offered skill among the skills not loaded in the uncertainty checklist. It ends with a JSON block holding what the submit tool would have
taken ([Agents](LOCAL.md#agents)). `git_log`, `git_blame` and `try_repro` have no
local counterpart, so a local expert hands in its reproduction and the verifier runs it.

| Tool                                           | Gives the agent                                                               | Roles                            |
| ---------------------------------------------- | ----------------------------------------------------------------------------- | -------------------------------- |
| `git_diff`                                     | The change, for one file or all of them                                       | all but the summarizer           |
| `git_log`                                      | The history before the change                                                 | CI agent, planner, expert, judge |
| `git_blame`                                    | Who last changed a range of lines                                             | expert, fix agent, judge         |
| `list_skills`, `load_skill`, `read_skill_file` | The skills offered to it and their reference files                            | expert                           |
| `try_repro`                                    | One run of a reproduction test on a fresh copy, with the confirmed `test_one` | expert                           |
| `submit_plan`, `submit_report`, …              | The role's result                                                             | one per role                     |

The agent's own tools for reading files stay available. eyeful denies its requests to edit files or
run commands, so the only commands that run are eyeful's fixed ones. In the cloud, a submit tool
checks what it receives and returns the problems it finds, so the agent can correct them in the same
session; `submit_report` runs the same checks Go applies to a report. Locally the same checks run
when the reply arrives, and a rejected report goes back to the expert once. The L1 tools, `find_references` and
`lookup_standard` are planned.

Tools marked L1 below run during triage on every review, before any expert starts. The others run
only when an expert calls them.

## Keeping standards in view {#context}

Standards are long, and an agent working through a long context tends to lose track of what it read
early on. eyeful therefore keeps them out of the prompt. The expert loads the skills it is offered
and reads a skill's reference files only when it needs them.

1. An expert reviews one change group per session, with that group's files, the plan and the skills
   offered for them. A group never spans more than one [scope](PLANNER.md#large).
2. Each finding names the skill it relies on, or `other` with a short category and, if one applies,
   a CWE identifier. A bug backed by a reproduction or a trigger path also needs a scenario: the
   input that triggers it and what goes wrong. A finding that cites a skill not offered to the expert is rejected, which stops
   an expert from inventing a standard. Go also rejects a finding on a file outside the expert's
   group, without a line or a subject, with an evidence kind its expert file does not list, or with
   `cve`, `api_break` or `coverage` evidence that no `dependency_audit`, `openapi_diff` or
   `coverage_diff` hit on that file backs. A rejected report goes back to the expert once with the
   reasons.
3. eyeful records which skills each expert loaded with `load_skill`. A skill it was offered and did
   not load is listed in the uncertainty checklist, since its standard may not have been applied.
   Locally every offered skill is in the prompt, so all of them count as loaded.

## design

The design expert asks whether the change belongs where it is and fits the rest of the system:
whether its parts work together sensibly, whether it belongs in a library instead, whether a new
abstraction is needed now or only in case, and whether a function or type is harder to follow than
necessary.

It works from Google's questions on design and complexity, coupling and cohesion, the SOLID
principles, McCabe's cyclomatic complexity, and the project's architecture decision records if there
are any. Its skills are `codebase-design`, plus
`golang-design-patterns` for Go files, `python-design-patterns` for Python files and
`nodejs-backend-patterns` for JavaScript and TypeScript files, and it reads the project's `ARCHITECTURE` document when one
exists. Its tools are `complexity` (L1, lizard on the changed
functions), `dependency_graph` (which packages now depend on which) and `find_references`.

Most design findings rest on an argument, so they become `suggestion`, `question` or `thought`
comments, unless a metric crosses the project's threshold. Every design comment names the principle
it relies on.

## correctness

The correctness expert asks whether the change does what its author meant it to: boundary values,
empty and very large inputs, error paths, concurrency (races, deadlocks, ordering), resource leaks,
and behaviour at each call site that `find_references` turns up.

It works from Google's questions on functionality, the SEI CERT coding standard for the language
where there is one, and CWE for classifying each defect. Its skills are `code-review` and `tdd`, plus
`golang-error-handling` and `golang-concurrency` for Go files, `python-error-handling` and
`async-python-patterns` for Python files, and `typescript-advanced-types` for JavaScript and TypeScript files. Its tools are `typecheck` (L1:
`tsc --noEmit`, `go vet`, `mypy`, planned), `static_analysis` (`staticcheck` or ESLint's correctness
rules, by language, planned) and `try_repro`, which runs a proposed reproduction test once so the
expert can check that it fails for the reason it claims.

When it suspects a functional error, it writes a test that should fail on the change. The test is how
the finding is proved, and the verifier is what runs it
([Evidence and verification](VERIFICATION.md)).

## security

The security expert asks whether the change introduces a vulnerability: injection, broken access
control, unsafe deserialisation, SSRF, leaked secrets, or dependencies with known vulnerabilities.

It works from the OWASP Top 10, OWASP ASVS for the requirement each finding breaks, the CWE Top 25,
and the SEI CERT coding standard where there is one. Its skills are `security-review`, whose references
follow the OWASP Cheat Sheet Series and cover JavaScript and Python, plus `golang-security` for Go
files. Its tools are `secret_scan` (L1, gitleaks over the diff),
`dependency_audit` (L1: `pnpm audit`, `npm audit`, `govulncheck`, `pip-audit` or `cargo audit`,
depending on the lockfile), `cve_lookup` (OSV records for each added or upgraded dependency) and
`sast` (Semgrep's OWASP rules on the changed files).

A known CVE or a leaked secret is evidence on its own. Other findings need a reproduction test or a
trigger path. Every finding carries its CWE identifier.

## tests

The tests expert asks whether the change's tests are worth keeping: whether they fail when the code
breaks, whether they assert something useful, whether they could fail for unrelated reasons, and
which changed lines no test runs.

It works from Google's questions on tests, the test design techniques of ISO/IEC/IEEE 29119, the
test smells catalogue of van Deursen et al., and mutation testing. Its skills are `tdd`, plus `golang-testing`
for Go files, `python-testing-patterns` for Python files and `javascript-testing-patterns` for
JavaScript and TypeScript files, and it follows the project's own test conventions. Its tools are `coverage_diff` (L1, the
`coverage` command compared line by line with the diff), `test_on_base` (runs the change's new tests
on the base commit, where a test of new behaviour should fail) and `mutate` (Stryker, mutmut or
go-mutesting on the changed lines only).

A new test that also passes on the base commit, or that still passes after the line it claims to
test is mutated, does not test anything new. Missing coverage is reported as a `suggestion`; this
expert does not write the missing tests.

## usability

The usability expert looks at the change from its users' side: people using the interface,
developers calling the API, and operators reading errors and logs.

It works from WCAG 2.2 and the WAI-ARIA Authoring Practices for accessibility, Nielsen's ten
usability heuristics and the ISO 9241-110 interaction principles for interaction design, RFC 9457
for error responses, and Semantic Versioning for API changes. Its skill is `accessibility`, which covers
WCAG 2.2 and is offered for markup and component files. Its tools are
`openapi_diff` (L1, breaking changes between the base and head API documents), `a11y_check` (axe on
the pages the change touches) and `e2e_run` (the `e2e` command, with screenshots).

A breaking API change or an axe violation is evidence on its own and is cited with its WCAG success
criterion. A finding based on a heuristic includes the screenshot and names the heuristic.

## readability

The readability expert asks whether the next engineer can read the change: whether names say what
things are, whether comments explain why rather than what, whether the code is consistent with its
surroundings, and whether documentation changes along with behaviour.

It works from the project's own style guide and contributing guide first, and otherwise from the
language's official conventions (Effective Go, PEP 8, the Google style guides), with Diátaxis for
documentation. Its skills are `golang-code-style` and `golang-naming` for Go files,
`python-code-style` for Python files and `modern-javascript-patterns` for JavaScript and TypeScript files; for
other languages it works from the project's style guide. Its tools are `lint` (L1, the project's linter on the changed files)
and `similar_code` (how the repository already does the same thing).

A point the style guide states becomes an `issue`. A point the guide does not cover is a `nitpick`
and does not block.

## An expert file

Each expert is one Markdown file under `workflow/prompts/experts/`, embedded in the binary. Its
frontmatter holds the roster entry and the skills; the body is the expert's prompt.
`workflow/prompts/roles/expert.md` adds what every expert shares: how to use the tools and what a
finding must contain. `security.md`:

```markdown
---
name: security
area: security
tier: strong
evidence: [repro_test, cve, trigger_path, argument]
skills:
  - name: security-review
  - name: golang-security
    files: ['**/*.go']
---

You are the security expert on a code review. You ask one question: does this change introduce a
vulnerability? Follow untrusted input from where it enters to where it is used. ...
```

`area` is the decoration its comments carry.

Adding an expert means adding a file. The [evaluation](EVALUATION.md#ablations) decides whether it
stays: if removing it changes no measured result, it is removed.

## What an expert sees

An expert receives the diff, the files the plan assigned to it, the plan, the L1 tool output for
those files, its skills, and the output of the tools it calls. It does not see other experts'
output, so when two experts report the same problem they arrived at it separately. Each expert runs
in its own process.

## When an expert fails

If an expert times out, fails, or sends a second report that Go rejects, it is dropped. The other experts carry on,
and the feedback lists the dropped expert as absent so the reader knows which part was not covered.
