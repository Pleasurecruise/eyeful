# 存储

云端不使用数据库。每次审查的状态在它的 Durable Object 里，阶段存档在 R2 里。任何地方都不保存凭据。

## Durable Object 存储

审查的 Durable Object 以 SQLite 为后端，在键 `review` 下保存一条记录：

| 字段         | 内容                                 |
| ------------ | ------------------------------------ |
| `source`     | pull request，格式与服务端读取的一致 |
| `status`     | `queued`、`running` 或 `done`        |
| `attempts`   | 为这次审查启动过的 Container 数      |
| `started_at` | 当前 Container 的启动时间，毫秒      |
| `outcome`    | 完成后工作流的结果                   |
| `reason`     | 审查没有结果就结束时的原因           |

## R2

bucket `eyeful-runs` 按 Durable Object 的 id 保存每次审查的阶段存档。服务端通过 `http://runs.eyeful` 访问它，Durable Object 把这个地址路由到 Worker 的 `Runs` 入口，且只限这次审查；Container 没有 R2 凭据。

| 键               | 内容                                         |
| ---------------- | -------------------------------------------- |
| `<id>/request`   | 工作流的请求，用来拒绝不同的请求             |
| `<id>/<stage>`   | 每个完成的阶段，比如 `plan/0` 或 `judge/…`   |
| `<id>/published` | 结果，在 GitHub 接受了 review 和评论之后写入 |

`internal/runs` 基于这些请求实现 `workflow.Archive`，所以 Container 重新启动时，工作流会跳过 R2 里已有的阶段。

## 测试 {#tests}

`internal/runs` 针对一个在内存里保存对象的 HTTP 服务测试。harness 有一个测试先让审查在发布时失败，再用一个一被调用就失败的 agent 重跑：第二次从保存的阶段完成并只发布一次。
