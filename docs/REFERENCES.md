# References

The standards eyeful reviews against, and the two projects it builds on.

## Software quality

- ISO/IEC 25010:2023, _Product quality model_. The experts are divided along its characteristics
  ([roster](EXPERTS.md#the-roster)).

## Code review {#code-review}

- Google Engineering Practices:
  [The Standard of Code Review](https://google.github.io/eng-practices/review/reviewer/standard.html)
  (when a comment blocks, [review standard](REVIEW.md#review-standard)),
  [What to look for](https://google.github.io/eng-practices/review/reviewer/looking-for.html)
  (the experts' questions; every line is read) and
  [How to write comments](https://google.github.io/eng-practices/review/reviewer/comments.html).
- [Conventional Comments](https://conventionalcomments.org). The format of every comment and the
  names of its labels and decorations ([comment format](REPORT.md#comment-format)). A community
  convention, so it names the labels and does not justify them.
- M. V. Mäntylä and C. Lassenius,
  ["What Types of Defects Are Really Discovered in Code Reviews?"](https://doi.org/10.1109/TSE.2008.71),
  _IEEE TSE_, 35(3), 2009. Review findings are functional or evolvability defects: the line between
  a `note` and the other severities ([comment format](REPORT.md#comment-format)).
- OASIS, [SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html). How findings
  are stored, and the two scales they are graded on: `kind` (§3.27.9) and `level` (§3.27.10)
  ([Kind and severity](VERIFICATION.md#kind-and-severity)).
- Tenny,
  [Trust Doesn't Live in the Code Review](https://raft.build/zh-cn/resources/blog/trust-doesnt-live-in-the-code-review/),
  Raft, 2026. A review is an event and trust is a state; a check counts only once it has been seen
  to fail ([limits](overview.md#limits), [rounds](REPORT.md#rounds), [tests](EXPERTS.md#tests)).

## Design and maintainability

- T. J. McCabe, "A Complexity Measure", _IEEE TSE_, 1976. Cyclomatic complexity.
- W. Stevens, G. Myers and L. Constantine, "Structured Design", _IBM Systems Journal_, 1974.
  Coupling and cohesion.
- R. C. Martin, _Design Principles and Design Patterns_, 2000. SOLID.
- M. Nygard,
  [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions), 2011.

## Correctness and security

- [SEI CERT Coding Standards](https://wiki.sei.cmu.edu/confluence/display/seccode).
- [CWE](https://cwe.mitre.org) and the [CWE Top 25](https://cwe.mitre.org/top25/). The weakness class
  of each correctness and security finding.
- [OWASP Top 10](https://owasp.org/Top10/) and
  [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/).
- [OSV](https://osv.dev) and [CVE](https://www.cve.org). Known vulnerabilities in dependencies.

## Testing {#testing}

- E. T. Barr, M. Harman, P. McMinn, M. Shahbaz and S. Yoo, "The Oracle Problem in Software Testing:
  A Survey", _IEEE TSE_, 41(5), 2015. Specified, derived, implicit and human oracles
  ([the oracle](VERIFICATION.md#oracle)).
- ISO/IEC/IEEE 29148:2018, _Requirements engineering_. Inspection, analysis, demonstration and test
  ([evidence](VERIFICATION.md#evidence-kinds)).
- C. E. Jimenez et al., "SWE-bench: Can Language Models Resolve Real-World GitHub Issues?", ICLR 2024. Fail-to-pass, pass-to-pass, and the statuses `PASSED`, `FAILED` and `ERROR`
  ([the verifier](VERIFICATION.md#the-verifier)).
- R. Just, D. Jalali and M. D. Ernst, "Defects4J", ISSTA 2014. A fault is tied to the change that
  introduced it: the `introduced` gate.
- C. Zhang et al., "SoK: DARPA's AI Cyber Challenge (AIxCC): Competition Design, Architectures, and
  Lessons Learned", arXiv:2602.07666, 2026. A proof of vulnerability, then a patch validated by
  reproducing it and running the functional tests: the two claims the gates support. A precedent,
  not where the gates come from.
- N. Alshahwan et al., "Automated Unit Test Improvement using Large Language Models at Meta", FSE
  Companion 2024. A generated test is kept only if it builds and gives the same result over five
  runs: the `runs` and `not_flaky` gates.
- ISO/IEC/IEEE 29119, _Software testing_. Test design techniques.
- A. van Deursen, L. Moonen, A. van den Bergh and G. Kok, "Refactoring Test Code", XP 2001. Test
  smells.
- Y. Jia and M. Harman, "An Analysis and Survey of the Development of Mutation Testing", _IEEE TSE_, 2011.
- Martian, "Code Review Bench", 2026, <https://github.com/withmartian/code-review-benchmark>. The
  offline set and the scoring [`match.py`](EVALUATION.md#tooling) follows.

## Usability and accessibility

- W3C, [WCAG 2.2](https://www.w3.org/TR/WCAG22/) and the
  [WAI-ARIA Authoring Practices Guide](https://www.w3.org/WAI/ARIA/apg/).
- J. Nielsen,
  [10 Usability Heuristics for User Interface Design](https://www.nngroup.com/articles/ten-usability-heuristics/).
- ISO 9241-110:2020, _Interaction principles_.
- [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457), _Problem Details for HTTP APIs_, and
  [Semantic Versioning 2.0.0](https://semver.org).

## Readability and documentation

- [Effective Go](https://go.dev/doc/effective_go), [PEP 8](https://peps.python.org/pep-0008/) and the
  [Google style guides](https://google.github.io/styleguide/), used when the project has no guide of
  its own.
- [Diátaxis](https://diataxis.fr). The four kinds of documentation.

## Projects

From [antfu/pulls.review](https://github.com/antfu/pulls.review) (MIT). eyeful runs
`@pulls.review/core` to group the change ([The plan](PLANNER.md#the-plan)) and shows its summary
first in the [terminal](REPORT.md#terminal). It also takes three ideas:

| Idea                                            | Where                             |
| ----------------------------------------------- | --------------------------------- |
| Sort files with rules before any model runs     | [Triage](PLANNER.md#triage)       |
| Read a working tree through a copy of the index | The [snapshot](LOCAL.md#snapshot) |
| Review a large change one scope at a time       | [Large changes](PLANNER.md#large) |

From [umputun/revmux](https://github.com/umputun/revmux):

| Idea                                                    | Where                                        |
| ------------------------------------------------------- | -------------------------------------------- |
| A fixed sequence of stages                              | [Stages](REVIEW.md#stages)                   |
| Merge findings within two lines and count their sources | The [ranked report](REPORT.md#ranked-report) |
| Carry on with less when a step fails                    | [Stages](REVIEW.md#stages)                   |
| Pass the previous round's findings to the next          | [Rounds](REPORT.md#rounds)                   |
| Keep a round's input beside its output                  | `snapshot.json` and `change.diff`            |
| One process per reviewer                                | Each [expert](EXPERTS.md) runs once          |
| A cap on concurrent reviewers and time limits per call  | [Agents](LOCAL.md#agents)                    |

## Compared with similar tools {#compared}

|                           | Claude Code ultrareview       | revmux                             | eyeful                                                         |
| ------------------------- | ----------------------------- | ---------------------------------- | -------------------------------------------------------------- |
| Who picks reviewers       | Not published                 | The caller picks a profile         | A planner groups the change; Go picks by rule                  |
| What is reviewed          | Bugs                          | What the profile's lenses cover    | Design, functionality, security, tests, usability, readability |
| How findings are verified | Reproduced in a cloud sandbox | Another agent reads the code       | The verifier runs a test; the rest is marked `review`          |
| Feedback                  | Findings, without fixes       | JSON or Markdown with a `fix` text | Conventional Comments in a ranked report                       |
| Where it runs             | Anthropic's cloud             | Inside the reviewed repository     | The user's machine, or a sandbox per pull request              |
