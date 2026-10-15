# 数据库与迁移

PostgreSQL 18 是唯一的数据存储。eyeful 用 pgx v5 连接数据库（`internal/db`），用 golang-migrate 执行内嵌的 `internal/db/migrations`，用 sqlc 从 `internal/db/queries` 生成 `internal/db/sqlc`。要改查询就改 SQL，然后运行 `mise run generate:db`；生成的代码不要手动修改。

## 迁移

第一个版本发布之前只有一个迁移文件 `0001_init`。表结构有变化时直接修改它，然后删库重建（`dropdb eyeful && createdb eyeful`）。发布之后，每次变更新增一对 `NNNN_name.up.sql` 和 `.down.sql`，已发布的迁移不再修改。

`eyeful-server migrate up|down|version` 用来手动执行迁移。`eyeful-server serve` 启动时会自动执行尚未应用的迁移，设置了 `--no-migrate` 时除外。

## 数据表

| 表        | 内容                                  |
| --------- | ------------------------------------- |
| `reviews` | 审查记录、幂等键、租约和 fencing 令牌 |

审查报告的存储要等报告结构定下来之后再加。

表名用复数的 `snake_case`。ID 是 Go 生成的带前缀的 `text`（`r_`）。时间字段一律用 `timestamptz`，每行都有 `created_at`，不保存任何凭据。

## 测试 {#tests}

需要 PostgreSQL 的测试，只有在 `EYEFUL_TEST_DATABASE_URL` 指向一个允许清空的数据库时才会运行。同一套存储接口测试会分别对 `reviewtest.MemoryStore` 和 `review.PostgresStore` 运行。
