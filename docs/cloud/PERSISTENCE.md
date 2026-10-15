# Database and migrations

PostgreSQL 18 is the only data store. eyeful connects with pgx v5 (`internal/db`), applies
migrations with golang-migrate from the embedded `internal/db/migrations`, and generates
`internal/db/sqlc` from `internal/db/queries` with sqlc. To change a query, edit the SQL and run
`mise run generate:db`; generated code is not edited by hand.

## Migrations

Until the first release there is a single migration, `0001_init`. Schema changes edit it, and the
database is dropped and recreated (`dropdb eyeful && createdb eyeful`). After the release, each
change gets its own `NNNN_name.up.sql` and `.down.sql` pair, and a released migration is not edited
again.

`eyeful-server migrate up|down|version` runs migrations by hand. `eyeful-server serve` applies pending migrations
at start-up unless `--no-migrate` is set.

## Tables

| Table     | Holds                                                    |
| --------- | -------------------------------------------------------- |
| `reviews` | Review records, idempotency key, lease and fencing token |

Report storage will be added once the report schema is designed.

Table names are plural `snake_case`. IDs are `text` values generated in Go with a prefix (`r_`).
Every time column is `timestamptz`, every row has `created_at`, and no credential is stored.

## Tests {#tests}

Tests that need PostgreSQL run only when `EYEFUL_TEST_DATABASE_URL` points to a database they are
allowed to wipe. A single store contract suite runs against both `reviewtest.MemoryStore` and `review.PostgresStore`.
