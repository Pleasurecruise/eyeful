# Questions and answers

## What exactly is reviewed?

A snapshot: a base commit and one commit holding the code under review, fixed before anything runs.

- Locally it is the uncommitted changes against `HEAD` by default. `--base` compares with a merge
  base, and `--head` reviews a commit or branch
  ([What can be reviewed](LOCAL.md#what-can-be-reviewed), [The snapshot](LOCAL.md#snapshot)).
- In the cloud it is always a pull request, and each push is a new snapshot
  ([Running in the cloud](cloud/CLOUD.md#what-can-be-reviewed)).

## What if the change is too large?

Triage sets aside any file whose diff is over 512 KB. When the rest has more than 2,000 changed
lines, Go splits it by directory into scopes, and each is planned and reviewed with a share of the
budget ([Large changes](PLANNER.md#large)).

## What if files change while a review runs?

The reviewed code does not change, because the snapshot was fixed first.

| Who changed them                     | What happens                                                              |
| ------------------------------------ | ------------------------------------------------------------------------- |
| The user, in the working copy        | Project commands are unaffected. eyeful reports the difference at the end |
| A project command                    | It runs in eyeful's checkout, which is reset after every run              |
| An agent                             | It has read-only tools. A change is reported at the end                   |
| A new push to a pull request (cloud) | It is a new review; the running one finishes on its own snapshot          |
