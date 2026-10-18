# Concurrency verification

Use for changes to the review's Durable Object, its alarm, the Container's start and stop, or the
stage archive in R2.

1. Name the orderings the change must survive before editing: a delivery sent twice, an alarm
   repeated after a failure, a Container stopped mid-review, a Container that never answers, a
   Container that finished but was not yet read.
2. Write a test per ordering that fails on the code before the change, then passes after it. In Go,
   `internal/harness` tests resume through `local/archive`, which stands in for `internal/runs` over
   R2. The Durable Object's decisions follow from its stored record, so state each ordering as the
   record before and after an alarm.
3. Run Go tests with `-race`.
4. For lifecycle changes, deploy, open a pull request, stop the Container during the review with
   `cf containers`, and keep the Worker's log of the alarm runs in `.agents/evidence/`.

A passing unit test does not show the alarm and the Container agree; only a deployed review does.
