# Storage

The cloud keeps no database. Each review's state lives in its Durable Object, and its stages live in
R2. Nothing stores a credential.

## Durable Object storage

The review's Durable Object is SQLite-backed and holds one record under the key `review`:

| Field        | Holds                                               |
| ------------ | --------------------------------------------------- |
| `source`     | The pull request, as the server reads it            |
| `status`     | `queued`, `running` or `done`                       |
| `attempts`   | Containers started for the review                   |
| `started_at` | When the current Container started, in milliseconds |
| `outcome`    | The workflow's outcome, once done                   |
| `reason`     | Why the review ended without an outcome, if it did  |

## R2

The bucket `eyeful-runs` holds each review's stage archive under the Durable Object's id. The server
reaches it through `http://runs.eyeful`, which the Durable Object routes to the Worker's `Runs`
entrypoint for that review only; the Container has no R2 credential.

| Key              | Holds                                                            |
| ---------------- | ---------------------------------------------------------------- |
| `<id>/request`   | The workflow request, so a different request is refused          |
| `<id>/<stage>`   | Each finished stage, such as `plan/0` or `judge/…`               |
| `<id>/published` | The outcome, written once GitHub accepted the review and comment |

`internal/runs` implements `workflow.Archive` over these requests, so the workflow skips every stage
already in R2 when a Container starts again.

## Tests {#tests}

`internal/runs` is tested against an HTTP server that keeps objects in memory. A harness test
runs a review whose publishing fails, then runs it again with an agent that fails if it is called:
the second run finishes from the saved stages and publishes once.
