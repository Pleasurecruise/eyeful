# Experts

Experts are the agents that review. Each covers part of the ISO/IEC 25010 quality model and has its
own skills. Go picks the experts each group of the plan needs ([The plan](PLANNER.md#the-plan)) and
runs each once over all of its groups, up to four at a time. The standards named here are listed in
[References](REFERENCES.md).

## The roster {#the-roster}

| Expert      | ISO/IEC 25010 characteristic               | Model tier |
| ----------- | ------------------------------------------ | ---------- |
| design      | Maintainability: modularity, modifiability | strong     |
| correctness | Functional suitability, reliability        | strong     |
| security    | Security                                   | strong     |
| tests       | Maintainability: testability               | mid        |
| usability   | Interaction capability                     | mid        |
| readability | Maintainability: analysability             | cheap      |
| skeptic     | Functional suitability, reliability        | strong     |
| generalist  | All of the above, alone                    | strong     |

- The skeptic reviews the same groups as correctness but is not shown the plan.
- The generalist runs only when named, as in `eyeful review generalist`, with every skill. It is the
  single-agent baseline of the [evaluation](EVALUATION.md).
- No expert covers performance yet ([Roadmap](ROADMAP.md)).
- The tier selects models through [provider routing](cloud/SCHEDULING.md#agent-runs-over-providers).

## Prompt, skills and tools {#skills-and-tools}

**Prompt.** An expert's prompt is the body of its file under `workflow/prompts/experts/`. The
planner, the fix agent, the reproduction agent, the judge and the CI agent have theirs under
`workflow/prompts/roles/`. Standards are not in the prompt.

**Skills.** A skill is a folder with a `SKILL.md`, in the Agent Skills format of
[skills.sh](https://skills.sh). eyeful writes none: it vendors widely installed ones unchanged under
`workflow/prompts/skills/`, with `skills-lock.json`, and embeds them in the binary.

- eyeful refuses to start when a skill's files do not match the hash in the lock file.
  `mise run skills` reinstalls them.
- A language skill is offered only when the expert's group holds a file of that language.
- A skill belongs to one expert of the panel. The generalist has all of them.

| Skill                         | From                                | Expert, and the files it is offered for                     |
| ----------------------------- | ----------------------------------- | ----------------------------------------------------------- |
| `code-review`                 | `anthropics/knowledge-work-plugins` | correctness                                                 |
| `golang-error-handling`       | `samber/cc-skills-golang`           | correctness, Go                                             |
| `python-error-handling`       | `wshobson/agents`                   | correctness, Python                                         |
| `typescript-advanced-types`   | `wshobson/agents`                   | correctness, JavaScript and TypeScript                      |
| `golang-concurrency`          | `samber/cc-skills-golang`           | skeptic, Go                                                 |
| `async-python-patterns`       | `wshobson/agents`                   | skeptic, Python                                             |
| `security-review`             | `getsentry/skills`                  | security                                                    |
| `golang-security`             | `samber/cc-skills-golang`           | security, Go                                                |
| `codebase-design`             | `mattpocock/skills`                 | design                                                      |
| `golang-design-patterns`      | `samber/cc-skills-golang`           | design, Go                                                  |
| `python-design-patterns`      | `wshobson/agents`                   | design, Python                                              |
| `nodejs-backend-patterns`     | `wshobson/agents`                   | design, JavaScript and TypeScript                           |
| `tdd`                         | `mattpocock/skills`                 | tests                                                       |
| `golang-testing`              | `samber/cc-skills-golang`           | tests and the reproduction agent, Go                        |
| `python-testing-patterns`     | `wshobson/agents`                   | tests and the reproduction agent, Python                    |
| `javascript-testing-patterns` | `wshobson/agents`                   | tests and the reproduction agent, JavaScript and TypeScript |
| `accessibility`               | `addyosmani/web-quality-skills`     | usability                                                   |
| `golang-code-style`           | `samber/cc-skills-golang`           | readability, Go                                             |
| `golang-naming`               | `samber/cc-skills-golang`           | readability, Go                                             |
| `python-code-style`           | `wshobson/agents`                   | readability, Python                                         |
| `modern-javascript-patterns`  | `wshobson/agents`                   | readability, JavaScript and TypeScript                      |
| `git-commit`                  | `github/awesome-copilot`            | `eyeful commit` only                                        |

**Tools.** `workflow/prompts/tools.yaml` defines the tools of the cloud runtime:

| Tool                                           | Gives the agent                                | Roles                            |
| ---------------------------------------------- | ---------------------------------------------- | -------------------------------- |
| `git_diff`                                     | The change, for one file or all                | every role                       |
| `git_log`                                      | The history before the change                  | CI agent, planner, expert, judge |
| `git_blame`                                    | Who last changed a range of lines              | expert, fix agent, judge         |
| `list_skills`, `load_skill`, `read_skill_file` | The skills offered to it                       | expert, reproduction agent       |
| `try_repro`                                    | One run of a reproduction test on a fresh copy | reproduction agent               |
| `submit_plan`, `submit_report`, …              | The role's result                              | one per role                     |

A local review has no tool server ([Agents](LOCAL.md#agents)):

- The diffs and the commit messages are part of the input.
- The skills are written to a temporary directory, and the input lists their paths.
- One file beside the patch holds the commits that last touched each changed file and who last
  changed the replaced lines, in place of `git_log` and `git_blame`.
- The agent ends with a JSON block in place of a submit call.
- `try_repro` has no counterpart: the verifier runs the test
  ([the verifier](VERIFICATION.md#the-verifier)).

The agent keeps its own tools for reading files. eyeful denies its requests to edit files or run
commands. Every other tool named below is planned ([Levels](LEVELS.md#planned)).

## What an expert reports {#context}

An expert receives the diff, the files of its groups, the offered skills and, unless it is the
skeptic, the plan. It does not see other experts' output. With `execution` it is also told which
tests the change breaks ([Regressions](VERIFICATION.md#regressions)), and in a re-review which
earlier findings still fail ([Rounds](REPORT.md#rounds)).

It reads every changed hunk and reports every problem the change introduces, without proving it. A
finding has:

- a file, a line and a subject;
- `related`: the other changed places where the same problem shows;
- a severity: `error`, `warning` or `note`;
- an evidence kind its expert file allows;
- the skill it relies on, or `other` with a category and a CWE identifier where one applies;
- for `test` and `analysis` findings, a scenario: the input that triggers it, what happens and what
  should happen.

Go rejects a finding that lacks any of these, is on a file outside the expert's groups, or cites a
skill the expert was not offered. A rejected report goes back once. Findings still invalid are
dropped and listed in the uncertainty checklist, and so is a skill an expert was offered and did
not load. A local review cannot see which skills were read.

An expert that times out or fails is dropped and listed as absent, and the review's result is
`partial`.

## design

Asks whether the change belongs where it is and fits the rest of the system.

- Standards: Google's questions on design and complexity, coupling and cohesion, SOLID, cyclomatic
  complexity, the project's architecture records.
- Planned tools: `complexity`, `dependency_graph`, `find_references`.
- Most findings rest on `inspection` and become `suggestion` comments. Each names its principle.

## correctness

Asks whether the change does what its author meant: boundaries, empty and large inputs, error
paths, concurrency, resource leaks.

- Standards: Google's questions on functionality, SEI CERT, CWE.
- Planned tools: `typecheck`, `static_analysis`.
- It reports the call that shows the error: the inputs and the wrong result.

## skeptic

Covers the same ground as correctness without being told what the change is for.

- For each removed or rewritten line it looks for what relied on the old behaviour.
- For each added line it looks for the input, state or order of calls the author did not expect.
- Two experts reaching the same lines count as corroboration ([Report](REPORT.md#ranked-report)).

## security

Asks whether the change introduces a vulnerability.

- Standards: OWASP Top 10, OWASP ASVS, CWE Top 25, SEI CERT.
- Planned tools: `secret_scan`, `dependency_audit`, `cve_lookup`, `sast`.
- Every finding carries its CWE identifier.

## tests

Asks whether the change's tests are worth keeping.

- Standards: Google's questions on tests, ISO/IEC/IEEE 29119, the test smells of van Deursen et al.,
  mutation testing.
- Planned tools: `coverage_diff`, `test_on_base`, `mutate`.
- For each added or edited test it names the mistake in the code that would make the test fail. A
  test no mistake fails is a finding.
- Missing coverage is a `suggestion`. It does not write the missing tests.

## usability

Looks at the change as its users do: people at the interface, developers calling the API, operators
reading errors.

- Standards: WCAG 2.2, WAI-ARIA Authoring Practices, Nielsen's heuristics, ISO 9241-110, RFC 9457,
  Semantic Versioning.
- Planned tools: `openapi_diff`, `a11y_check`, `e2e_run`.
- It reads markup and component code. Nothing renders a page yet.

## readability

Asks whether the next engineer can read the change.

- Standards: the project's style guide first, then the language's conventions (Effective Go, PEP 8,
  the Google style guides) and Diátaxis for documentation.
- Planned tools: `lint`, `similar_code`.
- A point the project's guide does not cover is a `nitpick`.

## An expert file

Each expert is one Markdown file, embedded in the binary. `workflow/prompts/roles/expert.md` adds
what every expert shares.

```markdown
---
name: security
area: security
tier: strong
evidence: [test, analysis, inspection]
skills:
  - name: security-review
  - name: golang-security
    files: ['**/*.go']
---

You are the security expert on a code review. ...
```

`area` is the decoration its comments carry. Adding an expert means adding a file; the
[evaluation](EVALUATION.md#ablations) decides whether it stays.
