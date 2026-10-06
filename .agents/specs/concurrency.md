# Concurrency verification

Use for changes to leases, fencing, the worker pool, deleting reviews, rate limits, or the start and
stop lifecycle.

1. Name the orderings the change must survive before editing: a stale lease writing late, a delete
   racing a claim, a shutdown mid-review, a reaper racing a renewal.
2. Write a test per ordering that fails on the code before the change, then passes after it.
3. Run it with `-race`, and run store tests against PostgreSQL (`EYEFUL_TEST_DATABASE_URL`), not
   only the memory store.
4. For lifecycle changes, start `bin/eyeful-server serve`, send `SIGTERM` during work, and keep the hook
   order from the log in `.agents/evidence/`.

A passing unit test against the memory store alone does not show a PostgreSQL ordering is safe.
