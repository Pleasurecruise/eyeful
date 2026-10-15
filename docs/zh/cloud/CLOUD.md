# 云端运行

云端审查在自行部署的 eyeful 服务器上针对 GitHub pull request 运行。一个部署对应一个 GitHub App，没有用户账号。服务器和审查队列已经有了，webhook 和审查本身还在规划中。

## 可以审查什么 {#what-can-be-reviewed}

安装了 eyeful 的 GitHub App 的仓库里的 pull request，仅此而已。分支、提交和未提交的改动在本地审查（见[本地运行](../LOCAL.md)）。

## 云端审查怎样运行

1. pull request 事件到达 `POST /webhook`，带有 App 的 webhook secret 签名，每个 head 提交排入一次审查。项目命令由 eyeful 从 base 分支上的清单文件找出，所以 pull request 改不了在它身上运行什么。
2. eyeful 读取 head 提交的 GitHub Actions 运行。有失败的就以诊断结束审查（见[规划与分拣](../PLANNER.md#ci-check)）。
3. 工作进程克隆仓库并记录快照：base 和 head 提交，以及相对于它们 merge base 的 diff。新的推送是新的快照和新的审查。
4. 分拣在沙箱里运行，规划器制定计划。
5. 每个专家作为工作进程上的一个 pi 进程运行。
6. 验证器在沙箱里的全新副本上运行复现。
7. eyeful 的 GitHub App 把行内评论作为一次 `COMMENT` review 提交，把排序报告作为一条评论提交（见[审查反馈](../REPORT.md)）。

## pi 里的 agent {#agents-in-pi}

每个 agent 是一个 pi 进程，有全新的 `HOME`，没有保存的会话，对检出只有只读工具。被审查仓库里提交的指令会被忽略。模型密钥留在 pi 进程这边，在沙箱之外。

```sh
pi --print --mode json --no-session \
  --model anthropic/claude-sonnet-5-5 \
  --tools read,grep,find_references,secret_scan,dependency_audit,cve_lookup,sast,submit_report \
  --extension eyeful-pi-extension.js \
  --append-system-prompt workflow/prompts/experts/security.md \
  "Review the files the plan assigned to you"
```

`--tools` 把 agent 限制在它的专家文件允许的范围内。这些工具由扩展提供，每次调用在沙箱里运行一条固定的命令。模型来自[供应商路由](SCHEDULING.md#agent-runs-over-providers)。

## 沙箱 {#the-sandbox}

- 每次审查有一个沙箱。
- 任何凭据都不进入沙箱：没有 GitHub token、API 密钥或模型密钥。
- 它在 `setup` 期间可以访问包仓库，除此之外什么都访问不了。
- 每个复现在全新的副本上运行。
- 沙箱带有所属审查的标签，服务器启动时清理孤儿沙箱。

这些限制是 `.agents/POLARIS.md` 里的硬性指标。

## 凭据 {#credentials}

随 webhook 一起规划：

| 凭据                     | 由谁持有           | 用途                              |
| ------------------------ | ------------------ | --------------------------------- |
| App ID、私钥             | 服务器的环境变量   | 为每次审查签发 installation token |
| webhook secret           | 服务器的环境变量   | 校验 `X-Hub-Signature-256`        |
| installation token，短期 | 工作进程，仅在内存 | 克隆、读取 CI、发布审查           |
| 模型密钥                 | pi 进程            | 运行 agent，费用由部署者承担      |

这些凭据都不存进数据库、不写进日志，也不传进沙箱。每次审查记录它的 token、费用和分钟数。
