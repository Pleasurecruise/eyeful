# Levels

A review should cost about as much as the change needs. Today every review runs the same way: rules
sort the change, a planner groups it, the experts each group needs review it, and what they find is
verified. How closely a review looks is left to the models, and what it may spend is set by its
budget. The cheaper ways of reviewing that this page used to describe, routing by tool output and a
quick level, are planned and are not in the code.

## Who decides how closely to look {#choosing-the-level}

Nobody picks a level, and no rule about the size of a change picks one either, the way revmux runs
one default profile instead of asking. How closely a review looks is left to the models, in three
places:

- The planner marks the groups where a mistake costs the most as core, and a project can add to them
  with `risk` paths in `.eyeful/config.yml`. A core group gets the correctness, skeptic, security
  and design experts whatever its category ([The plan](PLANNER.md#the-plan)).
- Each expert reads until it has seen everything the change does to its files and reports every
  problem it finds, large or small. Nothing tells it to leave anything out.
- Each expert grades its own findings. An `error` or `warning` finding is verified and shown on its own; a
  `note` finding, which changes no behaviour, is not verified and is folded into the nitpicks at the
  end of the report ([Review feedback](REPORT.md#ranked-report)).

## Budget

A review has a budget in tokens and money, and each command the verifier runs has its own timeout.
There is no limit on total minutes, since a project's own tests may take longer than any fixed
limit. Go checks the budget before it starts each agent, so agents already running when it runs out
still finish. The planner and the experts may spend three quarters of it and the rest is kept for
verification and the summary, so a review whose experts read a great deal still verifies what they
found; at most four experts run at once, which bounds how far they can go over
([Running locally](LOCAL.md#agents)). From then on the review starts no more agents and returns what
it has verified, with result `partial`.

## Planned: layers below the experts {#planned}

Running every expert on every change is simple but expensive, and a linter or an audit already
catches many problems. The plan is to start an agent only when a cheaper check has found something
for it:

| Layer | Would run                                                                                             | Model cost |
| ----- | ----------------------------------------------------------------------------------------------------- | ---------- |
| L0    | The CI check, triage, matching `risk` paths and file types                                            | none       |
| L1    | `lint`, `typecheck`, `secret_scan`, `dependency_audit`, `openapi_diff`, `complexity`, `coverage_diff` | none       |
| L2    | The planner, the readability expert, turning tool output into comments                                | small      |
| L3    | The other experts, and reproduction                                                                   | most       |

L0 exists. None of the L1 tools is built, so nothing in `workflow` waits for their output: an
earlier version carried a quick level that reviewed only what a tool flagged, rules that sent a tool
hit to an expert, and evidence kinds a tool hit had to back, and all of it was removed because no
review could reach it. It comes back with the first tool that runs, together with a measurement of
what routing by tools loses against running the planned panel ([Evaluation](EVALUATION.md#ablations)).
