# 数据库与迁移

数据库是 Cloudflare D1，也就是托管的 SQLite。`eyeful-server` 通过
[REST 查询接口](https://developers.cloudflare.com/api/resources/d1/subresources/database/methods/query/)
访问它：`POST /accounts/{account_id}/d1/database/{database_id}/query`，用 `EYEFUL_D1_API_TOKEN` 授权。
一次查询提交 `sql` 和 `params`。占位符是 `?`，值以字符串绑定，再在 SQL 里转换类型。客户端读取
`result[].results`。只有 D1 返回[重试指南](https://developers.cloudflare.com/d1/best-practices/retry-queries/)
里点名的错误时才会重试，最多五次：`Network connection lost`、`storage caused object to be reset`、
`reset because its code was updated`。响应没有回来就保持为失败，已经提交的领取不会再发一次。

在 Cloudflare 上，同样的请求在 Container 里发往 `http://d1.eyeful`，由 Worker 在它的 D1 绑定上执行（见[容器访问 D1](DEPLOYMENT.md#d1-from-the-container)）。`packages/worker/cloudflare.config.ts` 绑定名为 `eyeful` 的数据库。审查进程是 `eyeful-server`。

## 迁移

第一个版本发布之前只有一个文件 `0001_init.sql`。表结构有变化时直接修改它，然后重新创建数据库
（`cf d1 delete <id>`，再 `cf d1 create eyeful`）。发布之后，每次变更新增一个文件，已发布的迁移不再修改。

`cf d1 migrations apply` 把已应用的文件记在 `d1_migrations`，格式和 Wrangler 相同。服务端自己不做迁移。

第一次 `cf deploy` 会创建数据库。`cf d1 list` 打印它的 id；在 `packages/worker` 下这样应用迁移：

```sh
pnpm exec cf d1 migrations apply <id> --dir ../../internal/db/migrations
```

## 数据表

| 表              | 内容                                  |
| --------------- | ------------------------------------- |
| `reviews`       | 审查记录、幂等键、租约和 fencing 令牌 |
| `d1_migrations` | `cf` 和服务端已经应用的迁移文件名     |

`reviews` 是 `STRICT` 表。ID 是 Go 生成的带前缀的 `text`（`r_`）。时间是 unix 毫秒。`created_at` 在审查入队时写入。租约截止时间来自工作进程自己的时钟，作为绑定参数送进语句。两个进程的时钟可能有偏差。不保存任何凭据。

领取是一条 `UPDATE`，更新最早排队的那一行（`ORDER BY created_at, rowid`）。D1 一次执行一条语句。这条语句里的 fencing 令牌比已存储的最大令牌大 1。`Renew`、`Finish` 和 `Release` 同时匹配审查 id 和令牌。过期的租约回到队列；超过 `--max-attempts` 次后，审查变为 `done`，结果为 `none`，原因是 `worker lost N times`。

## 测试 {#tests}

存储测试通过同一个 D1 客户端，把这些 SQL 发给一个由 SQLite 文件支撑的本地查询接口（`internal/db/dbtest`），带 `busy_timeout`，并使用 `-race`。同一套测试也对 `reviewtest.MemoryStore` 运行。`mise run dev:cloud` 用本地 D1 运行 Worker 和 Container。
