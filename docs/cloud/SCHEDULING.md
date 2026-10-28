# Review lifecycle

The reusable workflow `.github/workflows/review.yml`, which each repository calls from its own `eyeful.yml`, decides when a review runs. Each review is one job; at
most one runs per pull request at a time.

| Mechanism | Behavior                                                                                                                                                                                                                          |
| --------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Once      | A pull request opened, or marked ready for review, is reviewed. Drafts are not.                                                                                                                                                   |
| Command   | A comment that is `/eyeful`, or starts with `/eyeful` and a space or a line break, on an open pull request starts a review of its current head. The job reacts 👀 to the comment as it starts.                                    |
| Who       | Only the pull request's or the comment's author with the association `OWNER`, `MEMBER` or `COLLABORATOR` starts a review.                                                                                                         |
| Queue     | The `concurrency` group per pull request queues a review behind a running one; a newer command replaces one still queued.                                                                                                         |
| Closed    | A command on a closed pull request ends the job without a review.                                                                                                                                                                 |
| CI        | The review waits for the head commit's workflow runs, leaving out every run of its own workflow, so an earlier failed or cancelled eyeful run never counts as the project's CI.                                                   |
| Status    | Whichever event starts it, the job sets an `eyeful` commit status on the pull request's head: pending with a link to the run, then success when the review is posted, failure when it is not, or error when the job is cancelled. |
| Re-run    | Re-running the job starts the review again and posts only what is missing.                                                                                                                                                        |
| Timeout   | The job stops after 120 minutes.                                                                                                                                                                                                  |
| Sandbox   | Each review has its own sandbox container, removed when the job ends.                                                                                                                                                             |

## Not done yet

- The review waits for CI inside the job, which keeps a runner busy while the pull request's checks
  run.
- A review started by a command does not yet reuse the last review's findings.
