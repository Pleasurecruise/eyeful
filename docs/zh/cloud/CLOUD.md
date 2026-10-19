# 云端运行

云端审查在 Cloudflare 上针对 GitHub pull request 运行，在自己的 Container 里运行和本地审查相同的工作流。测试在单独的沙箱里运行。

## 能审查什么 {#what-can-be-reviewed}

装了 eyeful GitHub App 的仓库里、由 `EYEFUL_ALLOWED_USERS` 里的用户打开或 @ 的 pull request。分支、提交和未提交的改动在本地审查（见[本地运行](../LOCAL.md)）。

## 云端审查怎样运行

1. pull request 在打开或转为可审查时审查一次，之后每次有人评论 `@eyeful-review` 再审一次。何时审、审几次见[审查生命周期](SCHEDULING.md)。
2. Worker 校验 webhook 签名，把 pull request 交给它的 Durable Object，由后者启动运行 `eyeful-server serve` 的 Container。
3. 服务端克隆仓库，等 head 提交的 GitHub Actions 跑完，然后回一个 👀。有失败时审查以诊断结束（见[规划与分拣](../PLANNER.md#ci-check)）。
4. 服务端打开沙箱，用执行验证调用 `local/app.Review`，每个阶段都存进 R2（见[存储](PERSISTENCE.md)）。
5. App 把每个未被否定的 `issue` 作为行内意见放进一条 `COMMENT` review 发布，简短总结作为一条评论发布（见[审查反馈](../REPORT.md#line-comments)），然后在 R2 里记下 `published`。两者都在隐藏标记里带着这次审查的 id（`EYEFUL_REVIEW_ID`），服务端发布前先列出 pull request 上已有的内容，所以崩溃后重新启动的 Container 不会把任何一条发两次。

## agent 与模型 {#agents-and-models}

每个专家是 Container 里的一个 pi 进程，以非特权用户 `agent` 运行，对检出的代码只有只读工具：

```sh
pi --mode json --no-session \
  --tools read,grep,find,ls \
  --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve
```

镜像里带着 `packages/worker/pi/` 下 pi 的 `settings.json` 和 `models.json`：模型是账号 `default` AI Gateway 上 `volcengine` 自定义 provider 的 DeepSeek V4.1 Flash。pi 请求 `http://models.eyeful`，Durable Object 把它路由到 Worker 的 `Models` 入口，由它带上 `cf-aig-authorization` 把 `POST /chat/completions` 转发到 AI Gateway，其他请求一律不转发。Container 里没有模型密钥，也没有 gateway token。provider 返回 429 或服务端错误时，pi 会等待后接着同一轮继续，最多八次，从 5 秒开始翻倍，每次最多 60 秒；eyeful 只看 pi 最后的回答，所以恢复成功的运行照常计入。

[Jev](../ARCHITECTURE.md#jev) 以 Workers AI 上的 `typesafe/jev` 运行。服务端的分类器请求 `http://jev.eyeful`，Worker 的 `Jev` 入口通过 AI binding 经同一个 gateway 作答。

## 沙箱 {#the-sandbox}

每次审查有自己的沙箱 Container，审查结束时删除。它和 `eyeful review` 一样，setup 跑一次，每次测试用全新的副本。

- 里面没有任何凭据。它从 `github.com` 克隆；Worker 只放行对被审查仓库的拉取，并加上这个仓库的只读 token。
- 命令取自 base 提交，pull request 改不了要运行的东西。
- 目前它能访问整个互联网，不只是包仓库。

## 凭据 {#credentials}

| 凭据                      | 由谁持有                                | 用途                              |
| ------------------------- | --------------------------------------- | --------------------------------- |
| Webhook secret            | Worker                                  | 校验 `X-Hub-Signature-256`        |
| App ID、私钥              | Worker；Container 启动时传给它          | 为每次审查签发 installation token |
| Installation token        | 服务端，只在内存里                      | 克隆、读取 CI、发布审查           |
| 只读 token                | `Git` 入口，每次请求签发                | 沙箱克隆                          |
| AI Gateway token、账号 ID | Worker                                  | `Models` 入口                     |
| Provider 密钥             | AI Gateway，存在账号的 Secrets Store 里 | 调用上游模型                      |
| R2 和 Workers AI          | Worker 的 binding                       | 阶段存档、Jev                     |

服务端读完配置后会把自己环境里所有 `EYEFUL_` 变量删掉，git 和 pi 不会继承 App 私钥。但 Container 的启动环境（`/proc/1/environ` 里能看到）仍有它，所以 pi 通过 `setpriv` 以用户 `agent` 运行。内核只允许服务端自己的用户读这个文件和服务端的内存；`agent` 保留 `dac_override` 能力，仍然能读检出的代码和 eyeful 为它写的文件。
