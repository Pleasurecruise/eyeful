# Evidence and verification

Experts claim a problem exists. The verifier, plain Go code in `workflow`, runs code to check the
claim, and the judge, an agent, reads the claims that cannot be run. Neither decides whether a
finding is shown; they decide how much it can be trusted.

The verifier answers four questions about a finding:

1. Is the problem real? A test for it fails on the change, every time, for the reason claimed.
2. Did the change introduce it? The same test does not fail that way on the base commit.
3. Does the fix work? With the suggested fix the test passes and nothing else breaks.
4. Who says the test is right? The behaviour it expects comes from somewhere other than the agent.

Only a finding that passes the first, second and fourth can block.

## An example

The change makes `is_expired` return `now > session.expires_at`. An expert reports that a session is
still valid at the expiry second. A reproduction agent writes:

```python
def test_expired_at_exact_boundary():
    assert is_expired(Session(expires_at=100), now=100)
```

The test fails on the change and passes with `>=`. If the pull request says "a session expires at
`expires_at`", the finding is an `issue (blocking)` with the fix attached. If nothing says where the
boundary is, the same test becomes a `question` for the author.

## The oracle {#oracle}

A failing test shows that the code and the test disagree, not that the test is right. Each
reproduction therefore declares its oracle: where the expected behaviour comes from. The four kinds
are those of Barr et al.'s survey of the oracle problem ([References](REFERENCES.md#testing)). Which
of them may block is eyeful's own rule, and the [evaluation](EVALUATION.md#ablations) measures what
it changes.

| Oracle      | In eyeful                                                               | Result   |
| ----------- | ----------------------------------------------------------------------- | -------- |
| `specified` | Text the finding quotes: an issue, the description, documentation       | `fail`   |
| `derived`   | A test already in the repository that now fails                         | `fail`   |
| `derived`   | A new test that passes on the base commit                               | `review` |
| `implicit`  | A crash, an unhandled error, a data race, a sanitizer report, a timeout | `fail`   |
| `human`     | Only the agent's reading of the code                                    | `review` |

The verifier grades the oracle itself:

- `specified`: it looks the quote up in the named documentation file, or in the description and the
  commit messages. A quote it cannot find, one shorter than three words, or one from a file of code
  makes the oracle `human`.
- `implicit`: the failure text must come from the runtime: a panic, a signal, a race or sanitizer
  report, a timeout, or the name of an error raised. If the test file contains it, or it is a
  failed assertion or a test that could not be imported, the oracle is `human`.
- `derived`: an existing test is run as it stands. A new test is `derived` only when it passes on
  the base commit.

A `review` result cannot block. It becomes a `question` with the failing test attached.

These checks read text. They catch a mislabelled oracle; they do not show that a quote supports
what the test expects, so the reader of a blocking comment still checks the quote.

## Evidence {#evidence-kinds}

`evidence` says how a finding can be checked, in the verification methods of ISO/IEC/IEEE 29148
([References](REFERENCES.md#testing)):

| Method       | Produced by                    | Meaning                                      |
| ------------ | ------------------------------ | -------------------------------------------- |
| `test`       | correctness, skeptic, security | A test that calls the code can show it       |
| `analysis`   | skeptic, security              | The expert reasons through the path          |
| `inspection` | every expert                   | The expert reads the code against a standard |

The standard's fourth method, demonstration, is planned with the tool that opens pages
([Levels](LEVELS.md#planned)). An expert only says a test could show the problem; another agent
writes the test afterwards.

## Kind and severity {#kind-and-severity}

Both scales are those of
[SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html).

The kind (§3.27.9) says how settled a finding is. The verifier and the judge set it:

| Kind     | A comment has it when                                                               |
| -------- | ----------------------------------------------------------------------------------- |
| `fail`   | Its reproduction failed on the change and not on the base, by a `fail` oracle       |
| `review` | Nothing ran, the judge confirmed it, or its reproduction rests on a `review` oracle |
| `open`   | The judge disputes it                                                               |

Only a `fail` comment is an `issue (blocking)` ([Review feedback](REPORT.md#comment-format)).

The severity (§3.27.10) is the expert's estimate of the cost if the change is merged as it is:

| Severity  | Given to                                                                           |
| --------- | ---------------------------------------------------------------------------------- |
| `error`   | Data lost or exposed, an unusable system, a wrong result or crash on a common path |
| `warning` | The same on a rare path, or needing an unusual input or state                      |
| `note`    | Nothing behaves differently: a name, style, wording                                |

## Tests the change breaks {#regressions}

With `execution`, before any expert starts, the verifier runs the project's `test` command on the
change. If it fails, the command runs again on the base commit. What passes there and fails on the
change is a regression.

The experts that report `test` findings are given the failing tests and the end of the output. A
finding that explains one names that test as its reproduction, with a `derived` oracle. The feedback
lists the broken tests whether or not an expert explained them. Locally it says only that the suite
broke, because the workspace cannot tell one test of a command from another.

## The verifier {#the-verifier}

The verifier is the only part of eyeful that runs the project's code, and it runs only the commands
in [`.eyeful/config.yml`](CONFIGURATION.md#project). It takes findings by severity, then by how many
experts reported the same lines. `note` findings are not verified.

For each `test` finding a reproduction agent writes a test, always as a new file. The test must pass
these gates in order:

| Gate               | The verifier                                                     | If it does not hold          |
| ------------------ | ---------------------------------------------------------------- | ---------------------------- |
| `runs`             | runs the test on a fresh copy of the change                      | the judge reads the finding  |
| `not_flaky`        | runs it five times; all must end the same way                    | the judge reads the finding  |
| `fails_as_claimed` | looks for the `fails` text in the output                         | the judge reads the finding  |
| `introduced`       | runs it on the base commit; it must not fail with that text      | pre-existing, never blocking |
| `fail_to_pass`     | applies the suggested fix and runs the test                      | reproduced, no verified fix  |
| `pass_to_pass`     | runs the project's tests with the fix; none that passed may fail | reproduced, no verified fix  |

The gates come from SWE-bench (fail-to-pass, pass-to-pass), Defects4J (the base commit) and Meta's
TestGen-LLM (repeated runs) ([References](REFERENCES.md#testing)). Their order and the `fails` text
are eyeful's.

- `fails` is a short text the test prints when it fails for the claimed reason. Unless the oracle is
  `implicit` it must be a message the test file itself writes, so a text any failure prints, such
  as `FAIL`, is refused. The verifier looks for it in the last 8,000 characters of the output.
- When the first runs fail without that text, the reproduction agent is asked once for a corrected
  test.
- The fix is written by a separate agent that sees the finding and the test, not the expert's
  reasoning. A fix that edits the test is not verified.
- Findings of different experts on the same file within two lines are verified together, with one
  reproduction for the most severe. The comment takes its words from the finding whose reproduction
  held.
- When the test command does not name the tests that fail, as locally, a fix is verified only if
  the whole suite passes with it.
- A run starts from the snapshot's files. Files git ignores, such as build output, stay between
  runs, and an edit at an ignored path is refused.
- Every run is recorded as `PASSED`, `FAILED` (with the `fails` text) or `ERROR`, as in SWE-bench's
  harness.

When `setup` fails or the project has no test command, nothing runs and every finding goes to the
judge.

## The judge {#the-judge}

The judge reads what could not be run: every finding under `read_only`, and under `execution` the
findings no test showed. It calls a finding valid when all three hold:

1. the problem exists in the changed code;
2. the change introduces or exposes it;
3. it changes observable behaviour or breaks a stated contract.

One judge reads the findings of one plan group together, at most eight at a time, and up to four
judges run at once. A rejected finding is not removed: it stays in the feedback as `open`, folded
under "Disputed" with the judge's reason. The [evaluation](EVALUATION.md#labelling) labels findings
by the same three points.

## Findings as SARIF {#sarif}

Every finding is stored as a SARIF 2.1.0 result. The same file feeds the review comments, GitHub
code scanning and the evaluation.

| SARIF field        | Holds                                                                                                      |
| ------------------ | ---------------------------------------------------------------------------------------------------------- |
| `ruleId`, `taxa`   | The rule, and the CWE identifier where there is one                                                        |
| `kind`             | `fail`, `review` or `open`                                                                                 |
| `level`            | The severity of a `fail` result; `none` otherwise, as the standard requires                                |
| `baselineState`    | `new`, or `unchanged` when the base commit fails too                                                       |
| `message`          | The subject and discussion of the comment                                                                  |
| `locations`        | File and line                                                                                              |
| `relatedLocations` | Other places the problem shows                                                                             |
| `codeFlows`        | A security trigger path                                                                                    |
| `fixes`            | The suggested fix                                                                                          |
| `properties`       | `expert`, `severity`, `evidence`, `oracle`, `label`, `decorations`, `corroboratedBy`, `repro`, `objection` |

## Relation to CI {#relation-to-ci}

eyeful writes one test per finding and runs the project's own tests. It does not generate a suite or
measure coverage. For everything it does not run, the project's CI is the authority.
