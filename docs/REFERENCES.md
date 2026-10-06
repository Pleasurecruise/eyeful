# References

This page lists the standards eyeful reviews against and the two projects it took design ideas
from. Other pages link here when they mention them.

## Software quality

- ISO/IEC 25010:2023, _Product quality model_. The experts are divided along its quality
  characteristics, and each expert covers one or more of them ([roster](EXPERTS.md#the-roster)).

## Code review

- Google Engineering Practices,
  [The Standard of Code Review](https://google.github.io/eng-practices/review/reviewer/standard.html).
  Decides when a comment blocks: only when the change makes code health worse. Data comes before
  opinion, and the style guide decides questions of style ([review standard](REVIEW.md#review-standard)).
- Google Engineering Practices,
  [What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html).
  The questions each expert asks, and the rule that every line is read
  ([Planning and triage](PLANNER.md)).
- Google Engineering Practices,
  [How to write code review comments](https://google.github.io/eng-practices/review/reviewer/comments.html).
  Comments address the code rather than the author, give their reason and state their severity
  ([Review feedback](REPORT.md)).
- [Conventional Comments](https://conventionalcomments.org). The format, labels and decorations of
  every comment ([comment format](REPORT.md#comment-format)).
- OASIS, [SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html), the Static
  Analysis Results Interchange Format. Findings and their evidence are stored in it
  ([Findings as SARIF](VERIFICATION.md#sarif)).

## Design and maintainability

- T. J. McCabe, "A Complexity Measure", _IEEE Transactions on Software Engineering_, 1976.
  Cyclomatic complexity, which the design expert's `complexity` tool measures.
- W. Stevens, G. Myers and L. Constantine, "Structured Design", _IBM Systems Journal_, 1974.
  Coupling and cohesion.
- R. C. Martin, _Design Principles and Design Patterns_, 2000. The SOLID principles.
- M. Nygard,
  [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions), 2011. Architecture decision records, read when a project keeps them.

## Correctness and security

- [SEI CERT Coding Standards](https://wiki.sei.cmu.edu/confluence/display/seccode), for the languages
  that have one.
- [CWE](https://cwe.mitre.org) and the [CWE Top 25](https://cwe.mitre.org/top25/). The weakness class
  of each correctness and security finding.
- [OWASP Top 10](https://owasp.org/Top10/). The security expert's checklist of web application
  risks.
- [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/). Security
  requirements by level. eyeful's own sign-in also follows it ([Auth](AUTH.md)).
- [OSV](https://osv.dev) and [CVE](https://www.cve.org). Known vulnerabilities in dependencies.

## Testing

- ISO/IEC/IEEE 29119, _Software testing_. Test design techniques and terminology.
- A. van Deursen, L. Moonen, A. van den Bergh and G. Kok, "Refactoring Test Code", XP 2001. Test
  smells.
- Y. Jia and M. Harman, "An Analysis and Survey of the Development of Mutation Testing", _IEEE
  Transactions on Software Engineering_, 2011. Mutation testing, used to check whether tests fail
  when the code breaks.

## Usability and accessibility

- W3C, [WCAG 2.2](https://www.w3.org/TR/WCAG22/). Accessibility success criteria.
- W3C, [WAI-ARIA Authoring Practices Guide](https://www.w3.org/WAI/ARIA/apg/). Accessible patterns
  for common widgets.
- J. Nielsen,
  [10 Usability Heuristics for User Interface Design](https://www.nngroup.com/articles/ten-usability-heuristics/).
- ISO 9241-110:2020, _Interaction principles_. Suitability for the task, self-descriptiveness,
  conformity with expectations, learnability, controllability, use error robustness, user
  engagement.
- [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457), _Problem Details for HTTP APIs_, and
  [Semantic Versioning 2.0.0](https://semver.org). Error responses and breaking API changes.

## Readability and documentation

- [Effective Go](https://go.dev/doc/effective_go), [PEP 8](https://peps.python.org/pep-0008/) and the
  [Google style guides](https://google.github.io/styleguide/). Language conventions, used when the
  project has no style guide of its own.
- [Diátaxis](https://diataxis.fr). The four kinds of documentation, used to check that docs change
  along with behaviour.

## Projects

eyeful takes ideas about the shape of the workflow from two projects. It uses none of their code.

From [antfu/pulls.review](https://github.com/antfu/pulls.review), in planning and the report:

| Idea                                            | Where eyeful uses it                                                                        |
| ----------------------------------------------- | ------------------------------------------------------------------------------------------- |
| Sort files with rules before any model runs     | [Triage](PLANNER.md#triage): glob rules set aside lockfiles, generated code and the like    |
| Read the whole change before splitting it       | [The plan](PLANNER.md#the-plan): the planner reads the diff and writes what the change does |
| Constrain the plan with a schema                | The `submit_plan` schema and [Go's checks](PLANNER.md) on the plan                          |
| Group the change by intent                      | The plan's `groups`, which the [ranked report](REPORT.md) also sorts by                     |
| Name the part of the system a group touches     | Each group's `category`, from pulls.review's list                                           |
| Mark the groups that need extra care            | Each group's `core`, which experts review first and the report lists first                  |
| Show what the change does before any comment    | The summary and groups at the top of the [terminal output](REPORT.md#terminal)              |
| Read a working tree through a copy of the index | The [snapshot](LOCAL.md#snapshot) of uncommitted changes                                    |
| Review a large change one scope at a time       | [Large changes](PLANNER.md#large): scopes split by Go, files over 512 KB set aside          |

From [umputun/revmux](https://github.com/umputun/revmux), in the stages and the feedback:

| Idea                                                            | Where eyeful uses it                                              |
| --------------------------------------------------------------- | ----------------------------------------------------------------- |
| A fixed sequence of stages                                      | [Stages](REVIEW.md#stages)                                        |
| Merge findings on the same file within two lines, count sources | Merging and corroboration in the [ranked report](REPORT.md)       |
| Carry on with less when a step fails, instead of aborting       | Failure handling in [stages](REVIEW.md#stages)                    |
| Pass the previous round's findings to the next round            | [Rounds](REPORT.md)                                               |
| Keep a round's input beside its output                          | `snapshot.json` and `change.diff` in the [run](LOCAL.md#snapshot) |
| One process per reviewer, however much of the change it covers  | Each [expert](EXPERTS.md) runs once over all of its groups        |
| A cap on reviewers running at once, and time limits per process | Four at once; 2 minutes idle and 20 minutes in all per call       |

revmux's verify stage reads the code without running it. The [evaluation](EVALUATION.md) runs it as
an external reference.

## Compared with similar tools {#compared}

|                           | Claude Code ultrareview       | revmux                                           | eyeful                                                                        |
| ------------------------- | ----------------------------- | ------------------------------------------------ | ----------------------------------------------------------------------------- |
| Who picks reviewers       | Not published                 | The caller picks a profile with a fixed roster   | A planner for each change, checked by Go against the level                    |
| What is reviewed          | Bugs                          | What the profile's lenses cover                  | Design, functionality, security, tests, usability, readability                |
| How findings are verified | Reproduced in a cloud sandbox | Another agent reads the code and gives a verdict | The verifier runs a reproduction test; weaker findings are labelled as such   |
| Feedback                  | Findings, without fixes       | JSON or Markdown with a `fix` text per finding   | Conventional Comments: blocking ones on the line, the rest in a ranked report |
| Where it runs             | Anthropic's cloud             | Inside the reviewed repository, which it trusts  | The user's machine, or a cloud sandbox without credentials                    |
