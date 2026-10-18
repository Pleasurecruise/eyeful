# 云端运行

云端审查在 Cloudflare 上针对 GitHub pull request 运行，在自己的 Container 里运行和本地审查相同的工作流。沙箱还没有实现，所以它不运行项目的任何命令，发现由裁判核查。

## 能审查什么 {#what-can-be-reviewed}

装了 eyeful GitHub App 的仓库里、由 `EYEFUL_ALLOWED_USERS` 里的用户打开或 @ 的 pull request。分支、提交和未提交的改动在本地审查（见[本地运行](../LOCAL.md)）。

## 云端审查怎样运行

1. pull request 在打开或转为可审查时审查一次，之后每次有人评论 `@eyeful-review` 再审一次。何时审、审几次见[审查生命周期](SCHEDULING.md)。
2. Worker 校验 webhook 签名，把 pull request 交给它的 Durable Object，由后者启动运行 `eyeful-server serve` 的 Container。
3. 服务端克隆仓库，等 head 提交的 GitHub Actions 跑完，然后回一个 👀。有失败时审查以诊断结束（见[规划与分拣](../PLANNER.md#ci-check)）。
4. 服务端用只读验证调用 `local/app.Review`，和 `eyeful review --verification read_only` 一样，每个阶段都存进 R2（见[存储](PERSISTENCE.md)）。
5. App 把每个未被否定的 `issue` 作为行内意见放进一条 `COMMENT` review 发布，简短总结作为一条评论发布（见[审查反馈](../REPORT.md#line-comments)），然后在 R2 里记下 `published`。两者之间崩溃可能会发布两次；outbox 还在规划中。

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
