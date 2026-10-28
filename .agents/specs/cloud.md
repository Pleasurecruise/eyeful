# Cloud review verification

Use for changes to `action.yml`, the eyeful workflow, `cmd/eyeful-action`, `internal/` or the
sandbox.

1. Name the orderings the change must survive before editing: a job re-run after it posted part of
   the review, two mentions while a review runs, a push during the review, a cancelled job, CI still
   running when the review starts.
2. Write a test per ordering that fails on the code before the change, then passes after it. In Go,
   `internal/github` tests the markers and the wait against a fake API, and `internal/harness` runs a
   whole review against a fake agent and an in-process sandbox.
3. Run Go tests with `-race`.
4. For workflow or image changes, push to a branch, open a pull request, and keep the job's log in
   `.agents/evidence/`. Ask before triggering a paid review.

A passing unit test does not show the workflow, the images and the program agree; only a real job
does.
