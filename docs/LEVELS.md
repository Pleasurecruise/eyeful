# Levels

Every review runs the same way today: rules sort the change, a planner groups it, the experts each
group needs review it, and what they find is verified. The cheaper layers below are planned.

## Who decides how closely to look {#choosing-the-level}

Nobody picks a level. The models decide in three places:

- The planner marks the groups where a mistake costs most as core. A core group gets the correctness, skeptic, security and design experts
  ([The plan](PLANNER.md#the-plan)).
- Each expert reads everything the change does to its files and reports every problem it finds.
- Each expert grades its findings. `error` and `warning` findings are verified; `note` findings are
  folded into the nitpicks ([Review feedback](REPORT.md#ranked-report)).

## Budget {#budget}

A review has a budget in tokens and money, and each command the verifier runs has a timeout. There
is no limit on total time. Tokens are new input, output and cache writes. Tokens an agent reads back
from the provider's cache are recorded apart and do not count against the budget; they grow with
every turn an agent takes, so a review's bill is higher than its token count.

- Go checks the budget before starting each agent, so running agents still finish.
- The planner and the experts may spend three quarters; the rest is kept for verification.
- At most eight experts run at once ([Running locally](LOCAL.md#agents)).
- When the budget runs out the review starts no more agents and returns what it verified, with
  result `partial`.

## Planned layers {#planned}

The plan is to start an agent only when a cheaper check has found something for it:

| Layer | Would run                                                                                             | Model cost |
| ----- | ----------------------------------------------------------------------------------------------------- | ---------- |
| L0    | The CI check, triage and file types                                                                   | none       |
| L1    | `lint`, `typecheck`, `secret_scan`, `dependency_audit`, `openapi_diff`, `complexity`, `coverage_diff` | none       |
| L2    | The planner, the readability expert, turning tool output into comments                                | small      |
| L3    | The other experts, and reproduction                                                                   | most       |

Only L0 exists. Routing by tool output comes with the first L1 tool, together with a measurement of
what it loses against the full panel ([Evaluation](EVALUATION.md#ablations)).
