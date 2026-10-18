# 云端运行

云端审查在 Cloudflare 上针对 GitHub pull request 运行。一个部署对应一个 GitHub App，没有用户账号。每个 pull request 的 head 提交对应一次审查，每次审查都在自己的 Container 里运行和本地审查相同的工作流。沙箱还没有实现，所以云端审查不运行项目的命令，发现由裁判核查。

## 能审查什么 {#what-can-be-reviewed}

只能审查装了 eyeful GitHub App 的仓库里的 pull request。分支、提交和未提交的改动在本地审查（见[本地运行](../LOCAL.md)）。

## 云端审查怎样运行

1. pull request 在打开、重新打开或标记为可审查时自动审查一次。之后只有在评论里 `@eyeful-review` 才会再审；eyeful 先回一个 👀，再审查那一刻的 head。push 不会触发任何审查。
2. Worker 用 `@octokit/app` 校验 `X-Hub-Signature-256`，只为 `EYEFUL_ALLOWED_USERS` 里的 PR 作者或评论者行动，立即返回 `202`，并把 pull request 交给这个 PR 的 Durable Object。审查进行中时被 @，只回复"正在审查"，不做别的（见[审查生命周期](SCHEDULING.md)）。
3. Durable Object 启动这次审查的 Container，里面运行 `eyeful-server serve`，pull request 放在 `EYEFUL_REVIEW` 里。
4. 服务端签发 installation token，克隆仓库，等 head 提交的 GitHub Actions 全部跑完。如果有失败的，审查以诊断结束（见[规划与分拣](../PLANNER.md#ci-check)）。
5. 服务端用只读验证调用 `local/app.Review`：分拣、计划、专家、裁判和报告与 `eyeful review --verification read_only` 完全一样。每个阶段都存进 R2，重新启动的 Container 从最后完成的阶段继续（见[存储](PERSISTENCE.md)）。
6. eyeful 的 GitHub App 把每个未被否定的 `issue` 作为行内意见放进一条 `COMMENT` review 发布，简短总结作为一条评论发布（见[审查反馈](../REPORT.md#line-comments)），然后在 R2 里记下 `published`。此后重新启动的 Container 直接返回，不再发布。如果 GitHub 已接受发布、记录还没写入时崩溃，可能会发布两次；outbox 还在规划中。

审查失败、Container 连续丢失三次，或改动为空时，Durable Object 会在 pull request 上评论原因。关闭 pull request 或把它转回 draft 会取消正在进行的审查。

## agent 与模型 {#agents-and-models}

每个专家是 Container 里的一个 pi 进程，对检出的代码只有只读工具：

```sh
pi --mode json --no-session \
  --tools read,grep,find,ls \
  --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve
```

镜像里带着 `packages/worker/pi/` 下 pi 的 `settings.json` 和 `models.json`：模型是账号 `default` AI Gateway 上 `volcengine` 自定义 provider 的 DeepSeek V4.1 Flash。pi 请求 `http://models.eyeful`，Durable Object 把它路由到 Worker 的 `Models` 入口，由它带上 `cf-aig-authorization` 转发到 AI Gateway。Container 里没有模型密钥，也没有 gateway token。

[Jev](../ARCHITECTURE.md#jev) 以 Workers AI 上的 `typesafe/jev` 运行。服务端的分类器请求 `http://jev.eyeful`，Worker 的 `Jev` 入口通过 AI binding 经同一个 gateway 作答。

## 沙箱 {#the-sandbox}

这些限制是 `.agents/POLARIS.md` 里的硬性要求，它们描述的沙箱还没有实现。在此之前，云端审查不运行任何项目命令，由裁判核查，报告里会说明这一点。

- 每次审查一个沙箱。
- 不进入任何凭据：没有 GitHub token、API 密钥或模型密钥。
- 只在 `setup` 时访问包仓库，其他一律不能访问。
- 每次复现都在全新的副本上运行。

## 凭据 {#credentials}

| 凭据                      | 由谁持有                                | 用途                              |
| ------------------------- | --------------------------------------- | --------------------------------- |
| Webhook secret            | Worker                                  | 校验 `X-Hub-Signature-256`        |
| App ID、私钥              | Worker；Container 启动时传给它          | 为每次审查签发 installation token |
| Installation token        | 服务端，只在内存里                      | 克隆、读取 CI、发布审查           |
| AI Gateway token、账号 ID | Worker                                  | `Models` 入口                     |
| Provider 密钥             | AI Gateway，存在账号的 Secrets Store 里 | 调用上游模型                      |
| R2 和 Workers AI          | Worker 的 binding                       | 阶段存档、Jev                     |

服务端读完配置后会把自己环境里所有 `EYEFUL_` 变量删掉，git 和 pi 不会继承 App 私钥。但 Container 的启动环境里仍有它，agent 的 `read` 工具可能读到 `/proc/1/environ`。
TODO(security)：改为把 App 私钥取进内存，而不是放在启动环境里传入。
