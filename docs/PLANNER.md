# Planning and triage

The first stage of a review checks whether the change is ready to review, sets aside files that need
no review, and decides which experts review the rest. It runs in three steps, cheapest first.
`workflow` implements the steps and every rule on this page. The L1 tools, the CI reading on GitHub
and the planner's prompt and `submit_plan` tool belong to the runtimes, which are not built yet.

```mermaid
flowchart LR
    ci{"CI check"} -->|failed| diagnose["CI agent<br/>cause and fix"]
    ci -->|passed| triage["Triage<br/>rules, no model"]
    triage --> tools["L1 tools"]
    tools --> planner["Planner agent"]
    planner -->|submit_plan| check{"Go checks the plan"}
    check -->|valid| run["Start the experts"]
    check -->|invalid| retry["Retry once,<br/>then the default set"]
```

## CI check {#ci-check}

If a change does not build or its own tests fail, most findings would repeat what CI already
reports. The stage therefore starts by checking CI.

In the cloud, eyeful reads the GitHub Actions workflow runs for the head commit. While any run is in
progress, the review waits. If all runs passed, or the repository has no workflows, the review
continues.

Locally there is no CI run to read. The verifier runs the `lint` command from
[`.eyeful/config.yml`](CONFIGURATION.md#project) and the L1 `typecheck` in the work directory. The
full `test` command runs only at deep, since a project's whole suite can take longer than the
review.

If the check fails, no expert starts. The CI agent, the only agent this step can start, reads the
logs of the failed jobs (or the local command output) and reports, for each failure, the workflow,
job and step, the cause, and how to fix it. The review ends with result `ci_failed`. When CI passes
the check costs nothing; when it fails it costs one agent run.

## Triage {#triage}

Triage sorts the changed files with rules and no model. Glob patterns pick out lockfiles, generated
code, vendored code, binary files and the `skip` paths from `.eyeful/config.yml`, and a size rule
sets aside any file whose diff is larger than 512 KB. These files go to
no expert and are listed in the feedback as not reviewed. Following Google's
[Every Line](https://google.github.io/eng-practices/review/reviewer/looking-for.html#every-line),
every other line must go to at least one expert. The quick level is the exception: there, only
lines that a tool flagged are reviewed ([Levels](LEVELS.md)).

The remaining files become a manifest with one line per file:

```text
path                      class   +lines  -lines
app/auth/session.py       code        12       3
app/auth/test_session.py  test         0       0
web/src/Login.svelte      code        40      18
package-lock.json         lock       310     120
commits: "fix session expiry at the boundary", "show the expiry on the login page"
```

Triage also runs the L1 tools for the project's languages (`lint`, `typecheck`, `secret_scan`,
`dependency_audit`, `openapi_diff`, `complexity`, `coverage_diff`) and attaches their output to the
manifest. These tools use no tokens, and their output is what the planner works from
([Levels](LEVELS.md)).

## Large changes {#large}

A change of more than 2,000 changed lines in the files triage kept is split into scopes before
anything is planned, following pulls.review's practice of reviewing a large change one scope at a
time. Go does the split from the paths alone, with no model, so the same change always gives the
same scopes, in the cloud and locally:

1. Files are sorted by path and grouped by their first directory, then by their first two. A group
   still over 2,000 lines after two levels is cut, in path order, into parts of at most 2,000 lines;
   a single larger file is a part on its own.
2. Neighbouring groups are packed together while the total stays within 2,000 lines, so a scope is
   never mostly empty.
3. Each scope, in order, gets its own plan and its own experts, with the scope's files as the
   manifest. The planner never sees a manifest larger than one scope, and Go checks each plan
   against its own scope.
4. Each scope may spend an equal share of the review's budget. A scope that uses up its share
   starts no more agents, the review ends `partial`, and the scopes after it still run.

The level is chosen once for the whole change ([Choosing the level](LEVELS.md#choosing-the-level)),
and verification and the summary run once over the findings of every scope. The report lists the
scopes in order, which the author can use to split the change into a stack
([The ranked report](REPORT.md#ranked-report)); eyeful never creates branches or pull requests.

## The plan

The quick level has no planner; rules send each tool result to an expert. At standard and deep, the
planner reads the manifest and the tool output rather than the whole diff, and opens a file's diff
only when it needs to. It groups files by what they are for and picks experts for each group. It
submits the plan with a `submit_plan` tool whose schema encodes the rules, so a malformed plan is
rejected before it reaches Go. Below is an example; the schema is not final.

```json
{
	"groups": [
		{
			"category": "fix",
			"summary": "Sessions expiring exactly at expires_at were treated as valid",
			"files": ["app/auth/session.py"],
			"experts": [
				{ "name": "correctness", "why": "boundary condition in is_expired" },
				{ "name": "security", "why": "session validity decides access" }
			]
		},
		{
			"category": "feature",
			"summary": "The login page shows when the session expires",
			"files": ["web/src/Login.svelte"],
			"experts": [{ "name": "usability", "why": "new text on the login page" }]
		}
	],
	"skipped": [{ "name": "readability", "why": "lint reported nothing and no names changed" }],
	"confidence": 0.8
}
```

The planner's prompt includes this table of typical pairings. It is guidance, and the planner may
depart from it:

| Change                                      | Usually needs                 |
| ------------------------------------------- | ----------------------------- |
| Only docs or comments                       | readability                   |
| A new module, a new dependency, a new layer | design, tests                 |
| Login, authentication, permissions          | correctness, security, tests  |
| Frontend components, user flows             | usability, readability, tests |
| Database migrations, data processing        | correctness, security         |
| A public API signature                      | design, usability             |
| Concurrency, locking, retries               | correctness, design           |

## How Go checks the plan

The plan is only data until Go accepts it. Go rejects a plan when:

- a file that triage kept belongs to no group;
- a group has no expert;
- a group lists a file that triage did not keep;
- an expert name is not in the [roster](EXPERTS.md#the-roster), or appears twice in one group;
- a group uses more experts than the level allows ([Levels](LEVELS.md));
- the confidence is outside 0 to 1.

A rejected plan goes back to the planner once, with the reasons. If the second plan is also
rejected, or the planner fails twice, the review uses the default experts for its level: one group
with every kept file and the roster's experts in order, up to the level's limit. The feedback says
so. If the plan's confidence is below 0.5, the level goes up one step. Go then adds the experts that
signals call for ([Levels](LEVELS.md)), starts the experts and sets their budgets; the planner has
no control over these.
