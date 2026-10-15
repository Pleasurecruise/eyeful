# 本地运行

本地审查在用户的机器上运行，通过 `eyeful review` 或[桌面应用](#desktop)。它不需要 eyeful 账号、服务器或模型密钥：agent 就是用户已经登录的编码工具。

## 命令

`mise run install` 把 `eyeful` 放到 `PATH` 上（见[本地开发](DEVELOPMENT.md#commands)）。

```sh
eyeful provider                             # 各个 agent：是否已连接、是否已安装
eyeful connect claude                       # 或 codex、pi
eyeful review                               # 未提交的改动；由规划器挑选专家
eyeful review security                      # 一个专家审查所有改动的文件
eyeful review --head HEAD --base main --verification execution
eyeful commit                               # agent 写提交信息，由你确认
```

`review` 接受：

| 参数               | 含义                                                                                                |
| ------------------ | --------------------------------------------------------------------------------------------------- |
| 专家名             | 跳过规划器，由这些专家审查分拣后留下的所有文件                                                      |
| `--base`、`--head` | 审查什么（见[下文](#what-can-be-reviewed)）                                                         |
| `--verification`   | `read_only`（默认）、`execution` 或 `none`                                                          |
| `--yes`            | 不询问，直接确认项目命令                                                                            |
| `--run`            | 运行目录。用同一个目录重新开始的审查会跳过已保存的阶段，并重新运行 agent 调用失败或被预算拒绝的步骤 |
| `--description`    | 一个包含作者描述的文件，比如 pull request 的标题和正文                                              |

token 预算（3,000,000）和每条项目命令的时限（10 分钟）是固定的。设置见[配置](CONFIGURATION.md#local)。

`commit` 提交工作区里的所有改动，包括钩子。已连接的 agent 用 `git-commit` skill 写提交信息，`-m` 可以自己指定。它从不推送。

审查除了 `.eyeful/runs/` 之外不向工作区写任何东西，而这个目录在 git 里忽略自己。

## 桌面应用 {#desktop}

桌面应用通过同样的 `local/app` 调用做命令能做的事：

| 窗口里                                 | 相当于                           |
| -------------------------------------- | -------------------------------- |
| agent 菜单                             | `eyeful provider`、`connect`     |
| 分支菜单：当前分支、其他 worktree      | 打开那个 worktree 的目录         |
| 比较对象：未提交的改动，或某个分支     | 不带参数，或 `--base`            |
| 自定义范围                             | `--base`、`--head`               |
| 专家，或由规划器决定                   | 专家名                           |
| 验证方式：裁判 agent、运行测试、不验证 | `--verification`                 |
| 列出项目命令的对话框                   | `[y/N]` 提示                     |
| diff 旁边的发现；日志标签页            | 打印出来的报告                   |
| 文件列表下方的 Review 或 Commit        | `eyeful review`、`eyeful commit` |

它打开启动时所在的仓库，或上一次显示的仓库。它从用户的登录 shell 读取 `PATH`，所以找到的 agent CLI 和终端里一样。窗口重新获得焦点时会重新读取 diff。

## 可以审查什么 {#what-can-be-reviewed}

`eyeful review` 在包含当前目录的仓库里运行。

| 审查对象                 | 参数                   | 基准                           | Head                         |
| ------------------------ | ---------------------- | ------------------------------ | ---------------------------- |
| 未提交的改动             | 无                     | `HEAD`                         | 已暂存、未暂存和未跟踪的文件 |
| 一个分支及其未提交的工作 | `--base main`          | `main` 和 `HEAD` 的 merge base | 已暂存、未暂存和未跟踪的文件 |
| 一个提交或分支           | `--head X`、`--base Y` | `Y` 和 `X` 的 merge base       | `X`                          |

使用 `--head` 时 head 必须已检出且工作区干净，因为 agent 读的是仓库里的代码。本地模式从不审查 pull request，也从不调用 eyeful 服务器（见[云端运行](cloud/CLOUD.md)）。

## 快照 {#snapshot}

在任何东西运行之前，eyeful 把审查对象固定为一个快照：基准提交，加上一个包含被审查代码的提交。对于未提交的改动，它从索引的一份副本构建这个提交，作者和日期固定，所以同样的改动总是得到同样的提交。

- 这个提交写入一个临时对象目录，该目录通过 `objects/info/alternates` 借用仓库的对象。仓库里不写入任何东西，目录在结束时删除。
- 运行目录保存 `snapshot.json` 和 `change.diff`。
- 审查结束时 eyeful 再取一次快照，不一样就说明：agent 可能读到了更新的代码。

## 本地审查怎样运行 {#how-a-local-review-runs}

1. eyeful 找到仓库并取得快照。
2. 它读取 [`.eyeful/config.yml`](CONFIGURATION.md#project)，检查请求，打印审查对象、专家、agent 和验证方式。
3. 在 `execution` 下，它用 `git worktree add --detach` 把快照检出到临时目录（关闭钩子），并打印将在那里运行的命令。用户要确认这些命令，除非同样的命令之前在这个仓库里确认过。检出目录在每次运行后重置，结束时删除。
4. 在 `execution` 下，CI 检查先运行 `setup`，再运行 `lint`（见[规划与分拣](PLANNER.md#ci-check)）。
5. 分拣给文件分类，规划器根据文件和最新的 50 条提交信息制定计划。
6. 在 `execution` 下，验证器运行 `test` 以及上一次审查的复现（见[回归](VERIFICATION.md#regressions)、[多轮审查](REPORT.md#rounds)）。
7. 专家进行审查。复现被编写并运行，其余的由裁判阅读（见[证据与验证](VERIFICATION.md)）。
8. 打印发现（见[终端里的输出](REPORT.md#terminal)），并保存到 `.eyeful/runs/<time>/`：`snapshot.json`、`change.diff`、`review.md`、`findings.json`、`results.sarif`、`result.json`、每个阶段的检查点，以及 `agents/` 下每次 agent 调用的提示词和原始输出。

## Agent {#agents}

eyeful 以非交互模式运行已连接 agent 的命令行工具，每次调用一个新进程，在仓库根目录下。提示词从标准输入传入。agent 以一个符合该角色 schema 的 JSON 块结束，缺失或不合法时重试一次。

| Agent  | 运行方式                                                                                                                                                                                                   | 只读的实现方式              | 模型                            |
| ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------- | ------------------------------- |
| claude | `claude -p --output-format stream-json --verbose --include-partial-messages --json-schema … --no-session-persistence --strict-mcp-config --setting-sources project --disable-slash-commands`               | `--tools Read,Grep,Glob`    | 档位：`haiku`、`sonnet`、`opus` |
| codex  | `codex exec --json --ephemeral --skip-git-repo-check --ignore-user-config --ignore-rules --disable hooks --disable plugins --disable apps --enable skip_host_skill_discovery -c project_doc_max_bytes=0 -` | `--sandbox read-only`       | Codex 的默认模型                |
| pi     | `pi --mode json --no-session --no-mcp --no-extensions --no-skills --no-prompt-templates --no-context-files --no-approve`                                                                                   | `--tools read,grep,find,ls` | pi 的默认模型                   |

- **隔离。** 每个工具运行时都不带用户的设置、钩子、MCP 服务器、扩展、skill 和 `AGENTS.md` 文件。eyeful 从 Claude Code 的环境里去掉 `ANTHROPIC_API_KEY`，避免审查被按 API 用量计费。
- **登录。** 每个工具使用它已有的登录。`connect` 和每次审查都会检查工具是否在 `PATH` 上。
- **Token。** eyeful 统计一次调用首次处理的内容：输入、输出和写入缓存的部分。缓存读取不计入。用量计在用户自己的订阅上。
- **时间。** 2 分钟没有任何输出、或运行满 20 分钟的调用会被停止。因无输出或工具出错而停止的调用会再运行一次。
- **并发。** 最多四个专家同时运行。

不为 agent 配置任何 MCP 服务器。

## 分类器 {#classifier}

`local/classify` 通过 [go-jev](https://github.com/mattn/go-jev) 向 [Jev](ARCHITECTURE.md#jev) 提问，密钥取自 [`EYEFUL_TYPESAFE_API_KEY`](CONFIGURATION.md#secrets)。请求里只有每条发现的文件、行号、标题和讨论，没有代码。

## 本地风险 {#local-risks}

| 风险                                       | 防护                                                                                                          |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| `.eyeful/config.yml` 里有恶意命令          | 命令只在用户确认后运行，有任何变化都要重新确认                                                                |
| eyeful 准备改动时触发了仓库的钩子          | eyeful 的 git 命令运行时关闭钩子                                                                              |
| 代码里的内容操纵 agent                     | agent 的 CLI 只有只读工具                                                                                     |
| 在 `execution` 下，被操纵的 agent 写出测试 | 没有防护：复现测试和修复都是 agent 写的代码，它们以用户的权限和环境变量运行。只对你信任的代码使用 `execution` |
| agent 给出的测试标识或修改是精心构造的     | 标识作为一个带引号的词传入；修改限制在仓库内、`.git` 之外；新文件从不覆盖已有文件                             |
| 项目命令造成破坏                           | 它只在 eyeful 对快照的检出里运行                                                                              |
| 仓库自带了确认记录                         | 确认记录保存在用户的配置目录里                                                                                |
| 超时的命令留下了进程                       | eyeful 杀掉整个进程组                                                                                         |
| 审查期间工作区发生变化                     | 快照先固定下来，结束时 eyeful 会报告差异                                                                      |
| agent 读取仓库之外的文件                   | 没有防护：CLI 的读工具能读到用户能读的所有文件                                                                |

确认过的命令以用户自己的权限运行。用户的机器上没有沙箱；别人的代码应该放到云端沙箱里（见[云端运行](cloud/CLOUD.md#the-sandbox)）。
