# Review lifecycle

The reusable workflow `.github/workflows/review.yml`, which each repository calls from its own `eyeful.yml`, decides when a review runs. Each review is one job; at
most one runs per pull request at a time.

| Mechanism | Behavior                                                                                                                  |
| --------- | ------------------------------------------------------------------------------------------------------------------------- |
| Once      | A pull request opened, or marked ready for review, is reviewed. Drafts are not.                                           |
| Mention   | A comment starting with `@eyeful-review` on an open pull request starts a review of its current head.                     |
| Who       | Only the pull request's or the comment's author with the association `OWNER`, `MEMBER` or `COLLABORATOR` starts a review. |
| Queue     | The `concurrency` group per pull request queues a review behind a running one; a newer mention replaces one still queued. |
| Closed    | A mention on a closed pull request ends the job without a review.                                                         |
| CI        | The review waits for every other workflow run on the head commit, and leaves out its own.                                 |
| Check     | The job is the check: it fails when the review fails. A run from a mention is not attached to the head commit.            |
| Re-run    | Re-running the job starts the review again and posts only what is missing.                                                |
| Timeout   | The job stops after 120 minutes.                                                                                          |
| Sandbox   | Each review has its own sandbox container, removed when the job ends.                                                     |

## Not done yet

- The review waits for CI inside the job, which keeps a runner busy while the pull request's checks
  run.
- A review triggered by a mention does not yet reuse the last review's findings.
- The images are built in every job instead of pulled from a registry.
