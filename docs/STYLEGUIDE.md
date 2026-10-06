# Code style and review

These are the rules for eyeful's own code. The Go rules come from the sources cited in the Go
section, and the review steps at the end are how a change is checked before it is called done.

## Structure

- Packages are named after the capability they own; no `utils`, `helpers`, `common`, `manager`,
  `service`. Dependency direction is in [Architecture](ARCHITECTURE.md#dependency-direction).
- Each package keeps its types, enums, sentinel errors and consumer interfaces in `types.go`.
- Constructors are plain functions with typed parameters. Only a composition root (`internal/app`,
  `local/app`) imports fx. There is no package-level state.
- Interfaces belong to their consumer and list only what it calls.
- HTTP handlers are CRUD, one file per resource, in the order create, list, get, update, delete.

## Naming

| What                             | Case                                                 | Example                         |
| -------------------------------- | ---------------------------------------------------- | ------------------------------- |
| Go packages, directories         | lowercase, one word                                  | `review`, `apperror`            |
| Go files                         | lowercase, `snake_case` only when needed             | `memory_test.go`                |
| Go exported / unexported         | `PascalCase` / `camelCase`                           | `NewWorkerPool`, `consoleRoute` |
| Go initialisms                   | one case                                             | `ID`, `URL`, `reviewID`         |
| Go receivers                     | one or two letters, consistent per type              | `s *MemoryStore`                |
| Go tests                         | `TestXxx`, one to three words; scenarios in subtests | `TestFencing`                   |
| JSON fields, query parameters    | `snake_case`                                         | `created_at`                    |
| URL paths                        | lowercase plural nouns, `kebab-case`                 | `/api-keys/{id}`                |
| Operation IDs, SDK functions     | `camelCase` verb + noun                              | `updateReview`                  |
| Schemas, TS types, components    | `PascalCase`                                         | `Review`, `ReviewTable.svelte`  |
| TS variables, functions          | `camelCase`                                          | `tokenKey`                      |
| Error codes, SQL                 | `snake_case`                                         | `review_not_found`              |
| Environment variables            | `UPPER_SNAKE_CASE`, `EYEFUL_` prefix                 | `EYEFUL_DATABASE_URL`           |
| CLI flags, mise tasks, npm names | `kebab-case` (`:` for task variants)                 | `--critical-rate-limit`         |
| Docs                             | `UPPERCASE.md`                                       | `SCHEDULING.md`                 |

Names are short and unambiguous, and let the package carry context (`review.Store`, not
`review.ReviewStore`).

## Helpers

Write logic where it is used. A separate function needs a reason: three or more call sites, an
external boundary, a framework callback, a named domain operation or invariant, or enough
complexity for its own tests. One-caller wrappers, renamed expressions and hidden control flow are
inlined. Before adding a cache, retry layer, switch or dependency, state what fails without it.

## Dependencies

Toolchains follow `latest` in `mise.toml`, which is the only place a tool version is declared.
`package.json` has no `devEngines` or `packageManager`, even when a generator adds one. Libraries
are added at their latest release and pinned exactly (`go get …@latest`; `saveExact` in `pnpm-workspace.yaml`, no `^` or `~`). Exceptions are
recorded in [Development](DEVELOPMENT.md#toolchain-notes). Upgrades are their own commits.

## Go

- gofmt, goimports and golangci-lint clean; exceptions go in `.golangci.yml`, never `//nolint`.
- No comments except `TODO(area): what` (one line, removed with the work), swag annotations and
  `//go:` directives. `grep -rn "TODO(" cmd internal workflow local apps` is the backlog.
- Errors wrap with `%w` and are checked with `errors.Is`/`As`; never turned into a plausible
  default. Client-facing errors are `apperror` codes; anything else is a logged 500.
- `context.Context` comes first on anything doing I/O, is never stored, and bounds every external
  call. No mutex across I/O. Log with `slog`, never secrets.
- CLI commands are kong structs with `help`, `default` and `env` tags and a `Run() error` method.

Unsafe patterns, from [Effective Go](https://go.dev/doc/effective_go),
[Code Review Comments](https://go.dev/wiki/CodeReviewComments) and the
[Google style guide](https://google.github.io/styleguide/go/), mapped from their TypeScript habit:

| TypeScript habit | Avoid in Go                                                  | Instead                                                                      |
| ---------------- | ------------------------------------------------------------ | ---------------------------------------------------------------------------- |
| `any`, `unknown` | `any`, `interface{}`, `[T any]`, `map[string]any`            | Concrete types; union constraints; `json.RawMessage`; `TODO(area): type TBD` |
| `as`             | `x.(T)` without `, ok`; unchecked enum conversion; `reflect` | `, ok`, type switches, `ParseX(string) (X, error)`                           |
| `try/catch`      | `recover()` outside the HTTP middleware; `panic`             | Return `error`; handle it once                                               |
| swallowed errors | `_ = f()`, `v, _ := f()`                                     | Handle or wrap and return                                                    |
| `undefined`, `!` | Unchecked nil; pointers only to mean "optional"              | Zero values; `(T, bool)`; pointers only for absence on the wire              |

## TypeScript and Svelte

- Start from the official scaffold and change only what eyeful needs.
- Vite+ runs everything: `vp dev`, `vp build`, `vp fmt` (Oxfmt), `vp lint` (type-aware Oxlint),
  `vp check`. Settings live in the root `vite.config.ts`.
- Types are checked with TypeScript 7. Wire types come from `@eyeful/sdk`, never hand-written.
- No `any`, non-null assertions or casts that skip validation.

## Security

Authentication, sessions and credentials follow current OWASP ASVS. Compare secrets in constant
time or store only their hash. Secrets do not enter sandboxes, logs, error responses or the
console bundle. Configuration and secrets live only at the repository root: secrets in `.env`, shared settings in
`mise.toml`; no app or package keeps its own env file, and code never falls back to a built-in value.

## Tests

Go tests sit next to the code (`*_test.go`, `go help test`), black-box (`package x_test`) unless a
seam is unexported, table-driven, with plain `if` assertions. Handlers are tested through
`httptest` against the real server. Network, credentials and databases are opt-in. A bug fix starts
with a failing test; concurrency tests cover the orderings that matter.

## Review

Before calling a change done, try to break it as:

1. a malicious user sending crafted input or skipping steps;
2. a producer of extreme data: empty, huge, duplicated, out of order, Unicode;
3. a user on an unstable network: timeouts, retries, double submits;
4. a user without permission: missing, wrong or expired credentials;
5. the new engineer who inherits the code.

A reviewer who did not write the change sees the problem before the author's diagnosis. Both write
down their conclusions separately, and a disagreement is settled by a test that tells the two apart.
Check that the change fixes a problem someone actually observed, keeps facts apart from inference,
contains nothing that could be removed, and keeps the hard measures in `.agents/POLARIS.md`. Every
handoff ends with an uncertainty checklist: what ran, what was not tested, what is still an
assumption, and what would settle it.
