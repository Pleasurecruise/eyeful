# API 约定

API 是基于 HTTP 的 JSON 接口，用 [echo v5](https://echo.labstack.com) 提供服务。从 handler 注解生成的 OpenAPI 3.1 文档是唯一的接口约定，SDK 和控制台都依据它生成。桌面端是本地客户端，不调用这个 API（见[本地运行](LOCAL.md)）。

## 约定

- 路由是针对复数资源的 CRUD：`POST` 创建，`GET` 列表，对 `/{id}` 的 `GET`、`PATCH`、`DELETE` 分别读取、修改、删除单个资源。状态变化通过 `PATCH` 某个字段完成，路径里不出现动词。
- JSON 字段用 `snake_case`，时间用 UTC 的 RFC 3339 格式，没有值的字段直接省略。
- 列表返回 `{ "items": [...] }`。`limit` 取值 1 到 100，默认 20。
- `POST /reviews` 返回 `202` 和一个排队中的审查，并接受 `Idempotency-Key`（按用户区分）。以后新增的非幂等 `POST` 也照此处理。
- 审查的来源是某个 GitHub 仓库的两个分支 `{ "repo", "base", "head" }`，指 `head` 相对它与 `base` 合并基点的改动。一个 pull request 就是这样一对分支。API 不接受本地路径，也不接受上传的代码。
- 请求体最大 1 MiB。ID 是不透明的字符串。
- 除公开路由外，所有路由都需要会话 cookie 或 API key（见[登录与凭据](AUTH.md)）。

## 错误

所有错误都是 [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457) 定义的 Problem，类型为 `application/problem+json`，带一个来自 `internal/apperror` 的固定 `code` 和 `request_id`。500 响应不包含底层错误，底层错误连同 request id 记录到日志里。

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

## 路由 {#routes}

| 资源         | 路由                                                                                                                    | 公开 |
| ------------ | ----------------------------------------------------------------------------------------------------------------------- | ---- |
| users        | `GET` `PATCH` `DELETE /users/me`                                                                                        |      |
| sessions     | `GET /sessions`、`GET` `DELETE /sessions/current`、`DELETE /sessions/{id}`                                              |      |
| api-keys     | `POST` `GET /api-keys`、`GET` `PATCH` `DELETE /api-keys/{id}`                                                           |      |
| reviews      | `POST` `GET /reviews`、`GET` `DELETE`（仅限排队中或已完成）`/reviews/{id}`                                              |      |
| repositories | `GET /repositories`、`GET /repositories/{owner}/{name}`、`…/commits/{ref}`、`…/commits/{sha}/diff`、`…/git/trees/{sha}` |      |
| oauth        | `GET /oauth/providers`、`GET /oauth/{provider}/authorize`、`GET /oauth/{provider}/callback`（`github`、`gitee`）        | 全部 |
| system       | `GET /health`、`GET /ready`、`GET /api/openapi.json`、`GET /api/docs`                                                   | 全部 |

仓库相关的响应是 GitHub 自己 schema（`full-repository`、`commit`、`git-tree`）的子集，字段名和必填字段都与 GitHub 一致，控制台可以直接按 GitHub 的结构处理。只能读取用户授权给 GitHub App 的仓库（见[仓库访问](AUTH.md#repository-access)）。

operation ID 采用 `createX`、`listX`、`getX`、`updateX`、`deleteX` 的形式，也就是 SDK 中的函数名。公开路由以 `METHOD /path` 的形式列在 `handlers.PublicRoutes` 里。

计划中：报告和发现（类型尚未设计）、SSE 进度推送、结果投递。

## MCP（计划中） {#mcp-planned}

`POST /mcp` 将把审查的 CRUD 作为 MCP 工具提供，用 API key 认证。它由官方 [Go SDK](https://github.com/modelcontextprotocol/go-sdk) v1.8.0 的一个无状态 `StreamableHTTPHandler` 提供，`SupportedProtocolVersions` 固定为：

| 版本         | 模式                                          |
| ------------ | --------------------------------------------- |
| `2025-11-25` | `initialize` 握手；客户端可以协商降到这个版本 |
| `2026-07-28` | 无状态：每个请求带 `_meta`，`server/discover` |

## 修改 API {#changing-the-api}

1. 修改 handler 及其 swag 注解。每个操作都要有 `@ID`。传输类型写在 `handlers/types.go` 里，用 `// @name X` 命名，始终存在的字段加 `validate:"required"`。
2. handler 负责把领域类型转换成传输类型，所以内部改名不会影响接口格式。
3. 运行 `mise run generate`，然后把 handler、`spec/` 和 `packages/sdk` 放在同一个提交里。三者不一致时 `mise run check` 会失败。
