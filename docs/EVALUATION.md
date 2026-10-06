# Evaluation

This page is part of the final-year project. It defines the experiment that answers the research
question, and its results are also the figures published about eyeful. The experiment runs in v1.0,
on the research core, before any other client is built ([Roadmap](ROADMAP.md)). The code will live
in `eval/` and be written in Python, which is not written yet. The three conditions below are
already a setting of `workflow`.

## Research question {#research-question}

For findings that can be reproduced, how many false positives does execution-based verification
with an external oracle remove compared with read-only verification, and what does it cost in
tokens, money and minutes?

The question covers only reproducible findings, meaning findings from the correctness and security
experts that come with a reproduction test. Design and readability findings are never executed, and
including them would dilute the effect. "Execution" means the verifier described in
[Evidence and verification](VERIFICATION.md), oracle rule included.

## Conditions

The main experiment is an ablation inside eyeful, so the prompts, the splitting of the change and
the models are the same in every condition:

| Condition | Experts and models | Verification                                                      |
| --------- | ------------------ | ----------------------------------------------------------------- |
| execution | Same               | The verifier runs each reproduction; strength follows the oracle  |
| read-only | Same               | A judge agent reads the code and each finding and gives a verdict |
| none      | Same               | Findings are reported as the experts wrote them                   |

A review picks its condition with `Request.Verification`: `execution`, `read_only` or `none`.

revmux, whose verify stage is read-only, also runs on the same subjects as an external reference.
It is not the baseline, because its prompts and splitting differ from eyeful's and those differences
would mix into the result.

## Datasets

| Dataset                       | Role                                                                                                                                                           |
| ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Injected-bug set              | Primary. Bugs injected into repositories and commits newer than the models' training cutoff, so the models cannot have seen them                               |
| SWE-bench Verified, reverted  | Secondary. Real bugs with tests that prove them, run in SWE-bench's own Docker environments. The models may have seen them in training, and the thesis says so |
| Code Review Bench offline set | Per-expert recall and precision compared with existing review tools                                                                                            |

Each subject in the first two sets has one known bug. Recall on that bug needs no labelling, but
everything else the experts report does.

## Labelling

Precision needs each additional finding labelled as either a real problem or a false positive. The
author does the labelling, following a written guideline (in the thesis appendix) that defines what
counts as a real problem. Findings from all three conditions are mixed and shuffled, and the
condition is hidden during labelling. About 20% of the findings are labelled a second time at least
two weeks later, and Cohen's κ between the two passes is reported as intra-rater agreement. Each
condition gets about 150 to 200 labelled findings. The thesis names single-annotator labelling as a
limitation and describes these measures.

## Ablations {#ablations}

Each part of the workflow has to show that it helps. It is removed, the experiment is rerun, and the
drop is measured.

| Removed                                             | Expected effect                                   | Compared on       |
| --------------------------------------------------- | ------------------------------------------------- | ----------------- |
| Execution verification (the main result)            | Precision on reproducible findings drops          | Both bug sets     |
| The oracle rule (any failing test is strong)        | Precision drops                                   | Both bug sets     |
| On-demand activation (every expert on every change) | Recall unchanged; cost and time rise              | Code Review Bench |
| The planner (rule routing at every level)           | Recall on mixed changes drops, or nothing changes | Code Review Bench |
| The summarizer (rule-based formatting)              | Feedback is less useful, or nothing changes       | Code Review Bench |
| Triage (all files go to experts)                    | Recall unchanged; cost rises                      | Code Review Bench |
| The CI check                                        | Recall unchanged; cost rises                      | Code Review Bench |
| Expert independence                                 | Fewer distinct findings per expert                | Injected-bug set  |
| Each expert in turn                                 | Recall in that expert's categories drops          | Code Review Bench |

A part whose removal changes nothing measured is removed from eyeful.

## Metrics and cost

The metrics are recall on known bugs, precision on labelled findings, planner accuracy, the share of
suggested fixes that pass, and tokens, money and minutes per review. Cost is measured only with pi,
where eyeful makes the model calls and counts them. Local reviews run on the user's own
subscription, and not every CLI reports cost, so they are left out of the experiment.

Every number comes from the run archive. For each review it holds every agent's prompt, output, tool
calls, model and token counts, and every verifier run with its command, exit code and duration. The
archive is part of the v1.0 research core, since without it there is no data.
