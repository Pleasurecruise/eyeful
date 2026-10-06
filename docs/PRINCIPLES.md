# Principles

These eight principles are the author's own summary of working habits. They are a reminder, not the
basis of a review. What a review does rests on the published literature listed in
[References](REFERENCES.md), above all Google's
[code review guide](https://google.github.io/eng-practices/review/) and
[Conventional Comments](https://conventionalcomments.org). Where a principle and the literature
differ, the literature wins.

| Principle                   | In development                                                               | In a review                                                                                                                                             |
| --------------------------- | ---------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| First principles            | Reproduce the problem and find its cause before changing code                | A blocking finding needs a test that fails before the fix and passes after, with an [oracle](VERIFICATION.md#oracle) other than the agent's own reading |
| Adversarial review          | Someone who did not build the change looks for counterexamples               | Experts did not write the change, and only the verifier records test results                                                                            |
| Ablation                    | A part whose removal changes nothing measurable is removed                   | An expert or stage stays only if the [evaluation](EVALUATION.md#ablations) shows it helps                                                               |
| Occam's razor               | Build the smallest version that works; add structure when a need arises      | [Levels](LEVELS.md): rules and tools run first, and models run only when something calls for them                                                       |
| Uncertainty checklist       | Each handoff lists what lacks evidence, what was not tested, what is a guess | Each report ends with absent experts, findings that were not reproduced, and runs that were skipped                                                     |
| Independent thinking        | Reach conclusions separately before comparing them                           | Experts do not see each other's output, so two experts agreeing counts as corroboration                                                                 |
| Critical thinking           | A complete explanation can still be wrong; say what would disprove it        | Each finding states its [evidence kind](VERIFICATION.md#evidence-kinds)                                                                                 |
| High cohesion, low coupling | Draw boundaries so the context a part needs is easy to state                 | Each agent gets its files, its checklist and the plan                                                                                                   |
