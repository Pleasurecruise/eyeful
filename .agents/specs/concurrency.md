# Concurrency verification

Use for changes to leases, fencing, the worker pool, or the start and stop lifecycle.

1. Name the orderings the change must survive before editing: a stale lease writing late, a duplicate
   admission racing another, a shutdown mid-review, a reaper racing a renewal.
2. Write a test per ordering that fails on the code before the change, then passes after it.
3. Run it with `-race`. Store tests send the D1 SQL through the D1 client to a local endpoint backed
   by SQLite, and run against the memory store as well. D1 runs one statement at a time.
4. For lifecycle changes, run `mise run dev:cloud`, send `SIGTERM` to the server in its Container
   during work, and keep the hook order from the log in `.agents/evidence/`.

A passing unit test against the memory store alone does not show the D1 statement ordering is safe.
`mise run dev:cloud` is what runs it against D1.
