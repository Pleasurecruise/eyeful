# 云端运行

云端审查在 GitHub Actions 的 job 里针对 pull request 运行，工作流和本地审查相同。agent 在一个容器里运行，项目的测试在另一个容器，也就是沙箱里运行。

## 能审查什么 {#what-can-be-reviewed}

workflow 里用了 eyeful action 的仓库中，由仓库所有者、成员或协作者打开、或评论了 `/eyeful` 的 pull request。分支、提交和未提交的改动在本地审查（见[本地运行](../LOCAL.md)）。

## 云端审查怎样运行

1. pull request 在打开或转为可审查时审查一次，之后每次有人评论 `/eyeful` 再审一次。何时审见[审查生命周期](SCHEDULING.md)。
2. job 按 action 的版本（例如 `main`）从 `ghcr.io` 拉取审查镜像和沙箱镜像，启动沙箱，在审查容器里运行 `eyeful-action`。
3. `eyeful-action` 读取 pull request，克隆它，等 head 提交上 eyeful 以外的 workflow 运行结束，然后回一个 👀。有失败时审查以诊断结束（见[规划与分拣](../PLANNER.md#ci-check)）。
4. 它打开沙箱，用执行验证调用 `local/app.Review`。
5. 它把每个未被否定的 `issue` 作为行内意见放进一条 `COMMENT` review 发布，简短总结作为一条评论发布（见[审查反馈](../REPORT.md#line-comments)）。两者都在隐藏标记里带着 workflow 运行的 id，发布前先列出 pull request 上已有的内容，所以重新运行 job 不会把任何一条发两次。

job 的日志会实时显示审查过程：审查什么、项目命令、每个阶段。完整的排序报告也会写进这次运行的摘要页面。

## agent 与模型 {#agents-and-models}

每个专家是审查容器里的一个 pi 进程，以非特权用户 `agent` 运行，对检出的代码只有只读工具：

```sh
pi --mode json --no-session \
  --tools read,grep,find,ls \
  --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve
```

镜像里带着 `cmd/eyeful-action/pi/` 下 pi 的 `settings.json` 和 `models.json`：模型是火山方舟上的 DeepSeek V4.1 Flash。pi 请求 `http://127.0.0.1:8081`，`eyeful-action` 在那里带上 `EYEFUL_MODEL_KEY` 把 `POST /chat/completions` 转发给这个 provider，其他请求一律不转发。pi 拿不到任何密钥。provider 返回 429 或服务端错误时，pi 会等待后接着同一轮继续，最多八次，从 5 秒开始翻倍，每次最多 60 秒；eyeful 只看 pi 最后的回答，所以恢复成功的运行照常计入。

[Jev](../ARCHITECTURE.md#jev) 用 `EYEFUL_TYPESAFE_API_KEY` 调用 TypeSafe 的 API。

## 沙箱 {#the-sandbox}

每次审查有自己的沙箱容器，job 结束时删除。它只运行一次 setup，每个测试都在新的副本上跑，和 `eyeful review` 一样。

- 它不持有任何凭证。它以只读方式挂载审查目录，从审查容器的检出里克隆这两个提交。
- 命令取自 base 提交，所以 pull request 改不了要运行的内容。
- 目前它能访问整个互联网，而不只是包仓库。

## 凭证 {#credentials}

| 凭证               | 持有者                           | 用途                     |
| ------------------ | -------------------------------- | ------------------------ |
| `GITHUB_TOKEN`     | 审查容器里的 `eyeful-action`     | 克隆、读取 Actions、发布 |
| 模型 provider 密钥 | `eyeful-action`，来自仓库 secret | 转发 pi 的模型请求       |
| TypeSafe API key   | `eyeful-action`，来自仓库 secret | Jev                      |

workflow 在 `pull_request_target` 和 `issue_comment` 上运行，即使 pull request 来自 fork 也能看到仓库的 secrets。它只检出 base 分支上的 eyeful，所以 pull request 改不了运行的 action；pull request 的代码只在沙箱里运行。

`eyeful-action` 读完配置后从自己的环境里删掉所有 `EYEFUL_` 变量，所以 git 和 pi 继承不到这些 token。容器的启动环境（`/proc/1/environ` 可见）里仍有它们，所以 pi 通过 `setpriv` 以用户 `agent` 运行。内核只允许 root 读这个文件和 `eyeful-action` 的内存；`agent` 保留 `dac_override` 能力，所以仍能读检出的代码和 eyeful 为它写的文件。
