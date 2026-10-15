# The server

`eyeful-server` is the cloud side: one Go binary, `cmd/eyeful-server`, with its packages in
`internal/` and the migrations embedded. Its only dependency is PostgreSQL. It shares the root
module with the local CLI and imports only `workflow` ([Modules](../ARCHITECTURE.md#layout)). It has no accounts, no
console and no public API; a GitHub App's webhook will be its only input. Reviews do not run on it
yet: the webhook, the pi runtime and the sandbox are not built, so workers end each review with a
placeholder executor ([Running in the cloud](CLOUD.md)).

## Components

```mermaid
flowchart TB
    github["GitHub App"] -.->|planned| server["server, handlers<br/>echo, health"]
    server -.->|planned| review["review<br/>records, leases"]
    scheduler["scheduler<br/>worker pool"] --> review
    review --> pg[("PostgreSQL")]
    scheduler -.->|planned| workflow["workflow"]
```

The `scheduler` runs workers that claim queued reviews. Dashed edges are planned.

## Routes and errors {#routes}

| Route           | Answers                         |
| --------------- | ------------------------------- |
| `GET /health`   | Liveness                        |
| `GET /ready`    | Readiness: the database answers |
| `POST /webhook` | Planned: pull request events    |

Request bodies are limited to 1 MiB. Every error is an
[RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) Problem with the type `application/problem+json`,
a stable `code` from `internal/apperror` and the `request_id`. A 500 response does not include
the underlying error; the error is logged with the request id.

## Layout {#layout}

| Path                 | Owns                                                           |
| -------------------- | -------------------------------------------------------------- |
| `cmd/eyeful-server`  | The server binary (kong): `serve`, `migrate`                   |
| `internal/app`       | The cloud composition root: fx wiring and lifecycle            |
| `internal/config`    | The `Config` struct, filled by kong                            |
| `internal/server`    | echo, middleware, Problem rendering                            |
| `internal/handlers`  | The routes                                                     |
| `internal/apperror`  | The Problem body and error codes                               |
| `internal/review`    | Review records and leases; `reviewtest` holds the memory store |
| `internal/scheduler` | The worker pool                                                |
| `internal/dispatch`  | Provider routing for agent runs (not wired in)                 |
| `internal/db`        | The pgx pool, migrations, SQL queries and sqlc output          |

Planned: `source` (the webhook), `checkout`, `pi`, `sandbox` and `sink` under `internal`.

## Dependency direction {#dependency-direction}

An arrow means "imports". Dependencies point inward from the entry point.

```mermaid
flowchart LR
    cmd["cloud"] --> app & config & db
    app --> server & handlers & scheduler & review & db
    handlers --> apperror
    server --> apperror
    scheduler --> review
    review --> sqlc["db/sqlc"]
    dispatch --> workflow
```

- Handlers translate HTTP and hold no business rules.
- `server` knows no route.
- Only `review` calls `internal/db/sqlc`.

## Lifecycle {#lifecycle}

[fx](https://github.com/uber-go/fx) is the dependency injection container. `internal/app` is the
only server package that imports it and owns every start and stop hook.

```mermaid
sequenceDiagram
    participant fx as internal/app
    participant db as PostgreSQL pool
    participant w as Workers
    participant http as HTTP listener
    Note over fx: Build: a failure exits early
    fx->>db: migrate, open
    Note over fx: Start
    fx->>w: start
    fx->>http: bind (a busy port fails start-up)
    Note over fx: Stop, in reverse, under one timeout
    fx->>http: drain
    fx->>w: release running reviews
    fx->>db: close
```

Shutdown runs in reverse. TODO(sandbox): sandbox and agent processes join the worker stop hook.

## Design decisions

1. The binary embeds what it needs: the migrations.
2. Packages follow the domain, and each owns its types, queries and rules.
3. SQL gives Go: the queries are generated, not written by hand.
4. Every background task has an owner and a way to stop.
5. A failure affects only the unit that failed and records why.
6. Waiting for capacity is not a failure, and committed output is not retried elsewhere.
7. PostgreSQL is the only database, because the queue relies on `SKIP LOCKED`.

Avoided: package-level state, goroutines without an owner, controller/service/model layers, and
hand-written CLI parsing.
