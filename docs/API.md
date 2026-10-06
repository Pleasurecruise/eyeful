# API

The API is JSON over HTTP, served with [echo v5](https://echo.labstack.com). The OpenAPI 3.1 document
generated from the handler annotations is the only contract, and both the SDK and the console are
built from it. The desktop app is a local client and does not call the API
([Running locally](LOCAL.md)).

## Conventions

- Routes are CRUD over plural resources: `POST` creates, `GET` lists, and `GET`, `PATCH` and `DELETE`
  on `/{id}` read, update and delete one item. A state change is a `PATCH` of a field; paths contain
  no verbs.
- JSON fields are `snake_case`, times are RFC 3339 in UTC, and absent values are left out.
- Lists return `{ "items": [...] }`. `limit` ranges from 1 to 100 and defaults to 20.
- `POST /reviews` returns `202` with a queued review and accepts an `Idempotency-Key` (scoped to the
  user). Any new `POST` that is not idempotent follows the same rule.
- A review's source is two branches of a GitHub repository, `{ "repo", "base", "head" }`, meaning
  what `head` changes relative to its merge base with `base`. A pull request is such a pair. The API
  does not accept local paths or uploaded code.
- Request bodies are limited to 1 MiB. IDs are opaque strings.
- Every route that is not public requires a session cookie or an API key ([Auth](AUTH.md)).

## Errors

Every error is an [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) Problem with the type
`application/problem+json`, a stable `code` from `internal/apperror`, and the `request_id`. A 500
response does not include the underlying error; the error is logged with the request id.

```json
{
	"type": "about:blank",
	"title": "Not Found",
	"status": 404,
	"code": "review_not_found",
	"detail": "review r_123 does not exist",
	"request_id": "8f1c…"
}
```

## Routes {#routes}

| Resource     | Routes                                                                                                                  | Public |
| ------------ | ----------------------------------------------------------------------------------------------------------------------- | ------ |
| users        | `GET` `PATCH` `DELETE /users/me`                                                                                        |        |
| sessions     | `GET /sessions`, `GET` `DELETE /sessions/current`, `DELETE /sessions/{id}`                                              |        |
| api-keys     | `POST` `GET /api-keys`, `GET` `PATCH` `DELETE /api-keys/{id}`                                                           |        |
| reviews      | `POST` `GET /reviews`, `GET` `DELETE` (queued or done) `/reviews/{id}`                                                  |        |
| repositories | `GET /repositories`, `GET /repositories/{owner}/{name}`, `…/commits/{ref}`, `…/commits/{sha}/diff`, `…/git/trees/{sha}` |        |
| oauth        | `GET /oauth/providers`, `GET /oauth/{provider}/authorize`, `GET /oauth/{provider}/callback` (`github`, `gitee`)         | all    |
| system       | `GET /health`, `GET /ready`, `GET /api/openapi.json`, `GET /api/docs`                                                   | all    |

Repository responses are subsets of GitHub's own schemas (`full-repository`, `commit`, `git-tree`),
with GitHub's field names and the same required fields, so the console can rely on GitHub's shape.
Only repositories the user has given the GitHub App can be read
([Repository access](AUTH.md#repository-access)).

Operation IDs follow the pattern `createX`, `listX`, `getX`, `updateX`, `deleteX` and become the SDK
function names. Public routes are listed as `METHOD /path` in `handlers.PublicRoutes`.

Planned: reports and findings (types not yet designed), SSE progress and deliveries.

## MCP (planned) {#mcp-planned}

`POST /mcp` will expose the review CRUD as MCP tools, authenticated with an API key. It will be
served by the official [Go SDK](https://github.com/modelcontextprotocol/go-sdk) v1.8.0 from a single
stateless `StreamableHTTPHandler`, with `SupportedProtocolVersions` pinned to:

| Version      | Model                                                |
| ------------ | ---------------------------------------------------- |
| `2025-11-25` | `initialize` handshake; clients negotiate down to it |
| `2026-07-28` | Stateless: per-request `_meta`, `server/discover`    |

## Changing the API {#changing-the-api}

1. Edit the handler and its swag annotations. Every operation has an `@ID`. Wire types live in
   `handlers/types.go`, are named with `// @name X`, and carry `validate:"required"` on fields that
   are always present.
2. Handlers map domain types to wire types, so renaming something internally does not change the
   wire format.
3. Run `mise run generate`, then commit the handler, `spec/` and `packages/sdk` together.
   `mise run check` fails if they drift apart.
