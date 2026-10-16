# Database and migrations

Cloudflare D1 is the database. D1 is managed SQLite. `eyeful-server` queries it with the
[REST query API](https://developers.cloudflare.com/api/resources/d1/subresources/database/methods/query/):
`POST /accounts/{account_id}/d1/database/{database_id}/query`, authorized with `EYEFUL_D1_API_TOKEN`.
A query sends `sql` and `params`. Placeholders are `?`, and the values are bound as strings and cast
in SQL. The client reads `result[].results`. It retries a query, up to five times, only when D1
returns an error named in the [retry guide](https://developers.cloudflare.com/d1/best-practices/retry-queries/):
`Network connection lost`, `storage caused object to be reset`, or `reset because its code was updated`.
A response that never arrives stays a failure, so a claim that already committed is not sent again.

On Cloudflare the same requests go to `http://d1.eyeful` inside the Container, and the Worker runs
them on its D1 binding ([D1 from the container](DEPLOYMENT.md#d1-from-the-container)).
`packages/worker/cloudflare.config.ts` binds the database `eyeful`. The review process is
`eyeful-server`.

## Migrations

Until the first release there is a single file, `0001_init.sql`. A schema change edits that file, and
the database is created again (`cf d1 delete <id>`, then `cf d1 create eyeful`).
After the release, each change gets its own file, and a released migration is not edited again.

`cf d1 migrations apply` records applied files in `d1_migrations`, in the same shape as Wrangler.
The server does not migrate.

The first `cf deploy` creates the database. `cf d1 list` prints its id; apply the files from
`packages/worker` with:

```sh
pnpm exec cf d1 migrations apply <id> --dir ../../internal/db/migrations
```

## Tables

| Table           | Holds                                                    |
| --------------- | -------------------------------------------------------- |
| `reviews`       | Review records, idempotency key, lease and fencing token |
| `d1_migrations` | Migration file names `cf` and the server have applied    |

`reviews` is a `STRICT` table. IDs are `text` values generated in Go with a prefix (`r_`). Times are
unix milliseconds. `created_at` is set when the review is admitted. A lease deadline is the worker's
clock, sent as a bound parameter. Two processes can disagree by their clock skew. No credential is stored.

A claim is one `UPDATE` of the oldest queued row (`ORDER BY created_at, rowid`). D1 runs one statement
at a time. The fencing token in that statement is one more than the highest token stored. `Renew`,
`Finish` and `Release` match the review id and the token. An expired lease returns to the queue, and
after `--max-attempts` the review is `done` with result `none` and the reason `worker lost N times`.

## Tests {#tests}

Store tests send this SQL through the same D1 client to a local query endpoint backed by a SQLite
file (`internal/db/dbtest`), with `busy_timeout` and `-race`. The same suite runs against
`reviewtest.MemoryStore`. `mise run dev:cloud` runs the Worker and the Container with a local D1.
