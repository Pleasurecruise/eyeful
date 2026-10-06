# Questions and answers

Short answers to questions that come up about what eyeful reviews. Each answer links to the document
that owns the rule.

## What exactly is reviewed?

A snapshot: a base commit and one commit that holds the code under review. Every review fixes its
snapshot before anything runs and keeps it with the findings.

Locally the snapshot comes from the repository the command starts in. By default it is the
uncommitted changes (staged, unstaged and untracked files) against `HEAD`; `--base` compares with a
merge base instead, and `--head` reviews a commit or branch. Uncommitted changes need no commit of
their own: eyeful records them as a commit that no branch points to. Local mode never reviews a
pull request. See [What can be reviewed](LOCAL.md#what-can-be-reviewed) and
[The snapshot](LOCAL.md#snapshot).

In the cloud the subject is always a pull request, and each push is a new snapshot
([Running in the cloud](CLOUD.md#what-can-be-reviewed)).

## What if the change is too large?

The same thing happens locally and in the cloud. Triage sets aside any file whose diff is larger
than 512 KB. When the files that remain have more than 2,000 changed lines, Go splits them by
directory into scopes of at most 2,000 lines, and each scope is planned and reviewed on its own with
a share of the budget. The split is code, not a prompt, so the same change always gives the same
scopes. The report lists the scopes in order, which the author can use to split the change into a
stack; eyeful never creates branches or pull requests itself. See
[Large changes](PLANNER.md#large).

## What if files change while a review runs?

The code eyeful reviews does not change, because the snapshot was fixed first. The rest depends on
who changed the files:

| Who                                  | What happens                                                                                                                                                                                           |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| The user, in the working copy        | Project commands are not affected; they run in eyeful's own checkout of the snapshot. The agents read the working copy, so when the review ends eyeful compares a new snapshot and says if it differs. |
| A project command, such as `setup`   | It runs in eyeful's checkout, which is reset after every run and removed at the end ([How a local review runs](LOCAL.md#how-a-local-review-runs)).                                                     |
| An agent                             | Its CLI runs with read-only tools only ([Agents](LOCAL.md#agents)). If a file changes anyway, the check at the end of the review reports it.                                                           |
| A new push to a pull request (cloud) | It is a new snapshot and a new review; the running one finishes on its own snapshot.                                                                                                                   |
