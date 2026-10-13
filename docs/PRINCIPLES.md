# Principles

Eight working habits of the author. They are a reminder, not the basis of a review: that is the
literature in [References](REFERENCES.md), and where the two differ the literature wins.

| Principle                   | In development                                                 | In a review                                                                      |
| --------------------------- | -------------------------------------------------------------- | -------------------------------------------------------------------------------- |
| First principles            | Reproduce the problem and find its cause first                 | A blocking finding needs a failing test with an [oracle](VERIFICATION.md#oracle) |
| Adversarial review          | Someone who did not build the change looks for counterexamples | Experts did not write the change, and only the verifier records test results     |
| Ablation                    | A part whose removal changes nothing measurable is removed     | An expert stays only if the [evaluation](EVALUATION.md#ablations) shows it helps |
| Occam's razor               | Build the smallest version that works                          | A path no review can reach is removed ([Levels](LEVELS.md))                      |
| Uncertainty checklist       | Each handoff lists what lacks evidence                         | Each report ends with absent experts, unreproduced findings and skipped runs     |
| Independent thinking        | Reach conclusions separately before comparing                  | Experts do not see each other's output                                           |
| Critical thinking           | Say what would disprove an explanation                         | Each finding states its [evidence kind](VERIFICATION.md#evidence-kinds)          |
| High cohesion, low coupling | Draw boundaries so a part's context is easy to state           | Each agent gets its files, its skills and the plan                               |
