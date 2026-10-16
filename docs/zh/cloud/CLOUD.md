# 云端运行

云端审查在自行部署的 eyeful 服务器上针对 GitHub pull request 运行。一个部署对应一个 GitHub App，没有用户账号。`POST /webhook` 为每个 head 提交排入一次审查，工作进程跑的是和本地相同的工作流。沙箱还没有实现，所以云端审查不运行项目命令，发现保持未验证。

## 可以审查什么 {#what-can-be-reviewed}

安装了 eyeful 的 GitHub App 的仓库里的 pull request，仅此而已。分支、提交和未提交的改动在本地审查（见[本地运行](../LOCAL.md)）。

## 云端审查怎样运行

1. pull request 事件到达 `POST /webhook`，带有 App 的 webhook secret 签名，每个 head 提交排入一次审查。项目命令由 eyeful 从 base 分支上的清单文件找出，所以 pull request 改不了在它身上运行什么。
2. eyeful 读取 head 提交的 GitHub Actions 运行。有失败的就以诊断结束审查（见[规划与分拣](../PLANNER.md#ci-check)）。
3. 工作进程克隆仓库并记录快照：base 和 head 提交，以及相对于它们 merge base 的 diff。新的推送是新的快照和新的审查。
4. 分拣在工作进程上运行，规划器制定计划。eyeful 在 base 提交上找到的命令记在这次审查上，但不会运行。
5. 每个专家是 `EYEFUL_AGENT` 指定的 agent 的一个进程（不设置时是 pi），对检出只有只读工具。
6. 验证器本应在沙箱里的全新副本上运行复现。沙箱还没有实现，所以 setup 会记下什么都没有验证，发现保持未验证。
7. eyeful 的 GitHub App 把行内评论作为一次 `COMMENT` review 提交，把排序报告作为一条评论提交（见[审查反馈](../REPORT.md)）。阶段存档旁边的 `published` 文件表示 GitHub 已经接受了这两次提交。重试时如果发现这个文件，就直接返回，不再发布。GitHub 已经接受、这个文件还没写上时进程崩溃，可能把评论再发一次。outbox 还在规划中。

## pi 里的 agent {#agents-in-pi}

每个 agent 是一个进程，有全新的 `HOME`，没有保存的会话，对检出只有只读工具。它的环境里没有 `GITHUB_TOKEN`、`GH_TOKEN`，也没有任何 `EYEFUL_` 变量。被审查仓库里提交的指令会被忽略。模型密钥留在这个进程里，在沙箱之外。pi 的命令是：

```sh
pi --mode json --no-session \
  --tools read,grep,find,ls \
  --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve
```

指令是标准输入上的提示词。`--tools` 是这份固定列表；专家文件里点名的扩展工具还在规划中。供应商路由已经写好，但还没有接入（见[agent 运行分配给模型提供商](SCHEDULING.md#agent-runs-over-providers)），所以每个角色都用 `EYEFUL_AGENT` 指定的那一个 agent。

## 沙箱 {#the-sandbox}

这些限制是 `.agents/POLARIS.md` 里的硬性指标。它们描述的沙箱还没有实现。在那之前，工作进程拒绝 `setup` 和每一条项目命令，并在报告里写明。

- 每次审查有一个沙箱。
- 任何凭据都不进入沙箱：没有 GitHub token、API 密钥或模型密钥。
- 它在 `setup` 期间可以访问包仓库，除此之外什么都访问不了。
- 每个复现在全新的副本上运行。
- 沙箱带有所属审查的标签，服务器启动时清理孤儿沙箱。

## 凭据 {#credentials}

| 凭据                     | 由谁持有           | 用途                              |
| ------------------------ | ------------------ | --------------------------------- |
| App ID、私钥             | 服务器的环境变量   | 为每次审查签发 installation token |
| D1 API token             | 服务器的环境变量   | 查询 D1                           |
| webhook secret           | 服务器的环境变量   | 校验 `X-Hub-Signature-256`        |
| installation token，短期 | 工作进程，仅在内存 | 克隆、读取 CI、发布审查           |
| 模型密钥                 | pi 进程            | 运行 agent，费用由部署者承担      |

这些凭据都不存进数据库、不写进日志，也不传进沙箱。每次审查记录它的 token、费用和分钟数。
