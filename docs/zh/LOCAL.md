# 本地运行

本地审查完全在用户的电脑上进行，可以用 `eyeful review` 命令或[桌面端](#desktop)发起。它不需要 eyeful 账号、服务端或模型密钥，因为 agent 就是用户已经登录的编码工具。

## 命令

`mise run install` 会把 `eyeful` 放到 `PATH` 上（见[本地开发](DEVELOPMENT.md#commands)）。

```sh
eyeful provider                             # 可用的 agent，哪个已连接，哪个已安装
eyeful connect claude                       # 也可以是 codex、pi
eyeful review                               # 由规划器挑选专家
eyeful review security                      # 由一个专家审查所有改动的文件
eyeful review correctness security --base main
eyeful commit                               # 由 agent 写提交说明，你确认后提交
```

`provider` 列出 eyeful 能驱动的每个 agent：是否是当前连接的那个，以及它的命令行工具是否在 `PATH` 上。`connect` 检查工具已经安装、读取它的版本，再保存下来。没有设置审查做多少的选项：eyeful 按每个改动自己选择档位，并且只在改动需要时才启动专家（见[档位](LEVELS.md#choosing-the-level)）。

指定了专家就跳过规划器，由这些专家审查分诊保留下来的所有文件；不写专家名时由规划器选择。`review` 只接受 `--base` 和 `--head`（审查什么）、`--verification`（默认 `read_only`，另有 `execution` 和 `none`）以及 `--yes`（确认项目命令）。token 预算（3,000,000）和每条项目命令的时限（10 分钟）是固定的。设置保存在哪里见[配置](CONFIGURATION.md#local)。

`commit` 提交工作区里的全部改动（无论是否已暂存），效果和 `git commit` 一样，也会运行 hook。提交说明由已连接的 agent 用 `git-commit` skill 根据 diff 写出（见[专家](EXPERTS.md#skills-and-tools)），agent 只读不写；你确认说明后由 eyeful 提交，`--yes` 跳过确认，`-m` 直接给出说明。它从不推送。

审查时，除了 eyeful 自己的 `.eyeful/runs/`（它在 git 里忽略自身），不往你的工作区写任何东西。`execution` 需要运行项目命令，这些命令只在[快照](#snapshot)的一份检出里运行，这份检出由 eyeful 自己创建、自己删除。

```sh
eyeful review correctness                                   # 未提交的改动；由裁判 agent 检查每条发现
eyeful review --verification execution                      # 同上，并实际运行每个复现测试
eyeful review --head HEAD --base main --verification execution
```

## 桌面端 {#desktop}

桌面端在窗口里做命令能做的事，调用的也是同样的 `local/app`：

| 窗口里                                     | 对应命令                                |
| ------------------------------------------ | --------------------------------------- |
| agent 菜单：每个 agent、已连接的、未安装的 | `eyeful provider`；选择一个即 `connect` |
| 分支菜单：当前分支、其他 worktree          | 打开那个 worktree 的目录                |
| 对比基准：仅未提交的改动，或某个分支       | 不加参数，或 `--base`                   |
| 自定义范围：基准和头部                     | `--base`、`--head`                      |
| 专家，或由规划者选择                       | 专家名                                  |
| 验证方式：评审智能体、运行测试、不验证     | `--verification`                        |
| 列出项目命令的对话框，运行或取消           | `[y/N]` 提示；没有 `--yes`              |
| 差异旁和对应行上的发现；日志标签页         | 打印的报告和终端输出                    |
| 文件列表下方的审查或提交；生成按钮         | `eyeful review`；`eyeful commit`、`-m`  |

它打开启动时所在的仓库；不在仓库里启动时（比如从访达打开），打开上一次显示的仓库。文件夹按钮可以打开其他仓库。从访达或桌面菜单启动的应用拿不到终端的 `PATH`，所以它启动时从用户的登录 shell 读取 `PATH`，找到的 agent 命令行工具和 `eyeful provider` 相同。默认显示当前分支未提交的改动；选择在其他 worktree 里检出的分支会打开那个 worktree，没有在任何地方检出的分支只能用作对比基准。改动的文件按 GitHub Desktop 的方式平铺列出，路径再深也不会遮住文件名。审查对象是未提交的改动时才能提交。文件列表和差异旁的审查面板可以调整宽度和折叠，开始审查时审查面板会展开。差异就是审查对象的差异，读取方式和审查相同。取消审查和按 Ctrl-C 的效果一样。运行目录及其中的文件和命令行相同。

## 可以审查什么 {#what-can-be-reviewed}

`eyeful review` 在当前目录所在的仓库里运行，通过 `.git` 找到仓库，没有仓库就停止。

| 审查对象                   | 参数                   | 基准                             | head                         |
| -------------------------- | ---------------------- | -------------------------------- | ---------------------------- |
| 未提交的改动               | 无                     | `HEAD`；新仓库没有基准           | 已暂存、未暂存和未跟踪的文件 |
| 一个分支连同它未提交的改动 | `--base main`          | `main` 与 `HEAD` 的合并基点      | 已暂存、未暂存和未跟踪的文件 |
| 一个提交或分支             | `--head X`、`--base Y` | `Y` 与 `X` 的合并基点，或 `HEAD` | `X`                          |

agent 读的是仓库里的代码，所以用 `--head` 时，head 必须是当前检出的提交，且工作区干净；否则 eyeful 会停下来，说明要先检出哪个分支。本地模式从不审查 pull request，不会访问 eyeful 服务端，不使用服务端上配置的密钥，也不向它发送任何代码、记录或凭据。pull request 在云端审查（见[云端运行](CLOUD.md)）。

## 快照 {#snapshot}

在运行任何东西之前，eyeful 先把审查对象固定成一个快照：基准提交，加上一个装着被审查代码的提交。用 `--head` 时，这个提交就是 head。对未提交的改动，eyeful 用一份 index 副本生成这个提交，做法和 pulls.review 读取工作区相同：加入所有未被忽略的文件，写出树对象，以固定的作者和时间、以 `HEAD` 为父提交提交它。这个提交写进下面说的临时对象目录，从不进入仓库，也没有任何分支、ref 或 index 指向它。作者和时间是固定的，所以同样的改动总是得到同一个提交。

运行目录把快照保存为 `snapshot.json`（仓库、基准、head 和提交），把 diff 保存为 `change.diff`，和发现放在一起，就像 revmux 把每一轮的输入和输出放在一起。

快照的 git 对象从不进入仓库。eyeful 把它们写到一个临时对象目录，这个目录通过 git 的 `objects/info/alternates` 借用仓库自己的对象；本次审查的所有 git 命令和项目命令都使用这个目录，审查结束时删除它。之后仓库的对象库和审查前完全一样。

审查结束时，eyeful 再取一次快照。如果和原来不同，或者用 `--head` 时 head 已不再是干净的当前检出，eyeful 会提示：agent 读的是工作区，所以可能看到了更新的代码。发现会保留，指向记录下来的快照。项目命令不受影响，因为它们在快照自己的检出里运行。

## 本地审查的步骤 {#how-a-local-review-runs}

1. eyeful 找到仓库，取快照，并检查 agent 将要读的代码就是被审查的代码。运行产出放在 `.eyeful/runs/`，里面有一个内容为 `*` 的 `.gitignore`，所以 git 看不到它，下一次审查的快照也不会包含它。
2. 从仓库读取 [`.eyeful/config.yml`](CONFIGURATION.md#project)（第 1 步已确认仓库里就是快照的内容），在启动任何 agent 之前检查这次请求（专家、档位、预算、glob），然后打印要审查的内容、快照、专家、档位、agent 和验证方式。用 `execution` 时，用 `git worktree add --detach` 把快照检出到系统临时目录（关闭 hook），打印将在那里运行的命令，由用户确认；同一个仓库里确认过完全相同的命令时不再询问。`--yes` 表示直接确认；既没有终端也没有 `--yes` 时，审查停止。eyeful 每次运行之后都会把这份检出恢复原状，审查结束时删除它。命令不会在其他任何地方运行。
3. 用 `execution` 时，CI 校验在这份检出里先运行 `setup`，再运行 `lint`。`lint` 失败时由 CI agent 诊断原因，审查到此结束（见[规划与分诊](PLANNER.md#ci-check)）。用 `read_only` 或 `none` 时不运行任何命令，所以没有 CI 校验。
4. 分诊给文件分类，再由规划器根据这些文件和范围内提交的说明（最新的 50 条）制定计划，和云端一样。
5. 每个专家、规划器、修复 agent 和汇总 agent，都是已连接 agent 的命令行工具的一次新运行（见 [agent](#agents)）。
6. 用 `execution` 时，验证器在这份检出里用 `test_one` 运行每个复现测试，用 `test` 运行整套测试；用 `read_only` 时，由裁判 agent 读代码，判断每条发现是否属实。
7. `eyeful review` 按 Claude Code ultrareview 的形式打印发现，Important 在前（见[终端里的输出](REPORT.md#terminal)）。它把 `snapshot.json`、`change.diff`、`review.md`、`findings.json`、`results.sarif`、`result.json` 和每个阶段的检查点写到 `.eyeful/runs/<时间>/`。

## agent {#agents}

eyeful 以非交互模式运行已连接 agent 自己的命令行工具，每次调用启动一个新进程，工作目录是仓库根目录，做法和 revmux、pulls.review 相同。提示词由 `workflow/prompts` 写好，从标准输入传进去，所以没有长度限制。提示词里带着该角色需要的东西：diff（规划器拿到的是分诊在其范围内保留的文件），以及提供给专家的每个 skill 的名字、描述和路径：审查期间 skill 写在运行目录里，审查结束就删除。agent 用自己的只读工具读用得上的 skill 和其他文件，最后给出一段符合该角色 schema 的 JSON，内容就是该角色本来要提交的结果（见[提示词、skill 和工具](EXPERTS.md#skills-and-tools)）。这段 JSON 缺失或不符合 schema 时，eyeful 附上原因再运行一次，仍然不行就算这次调用失败。eyeful 不给 agent 配置任何 MCP 服务：MCP 只由云端提供（见 [API 约定](API.md#mcp-planned)）。agent 不能运行命令，所以真正运行的命令只有经过确认的项目命令，而且只在快照的检出里运行；agent 交出复现测试，由验证器去运行。

| agent  | 运行方式                                                                                                                | 只读的保证                  | 结果                                               | 模型                                    |
| ------ | ----------------------------------------------------------------------------------------------------------------------- | --------------------------- | -------------------------------------------------- | --------------------------------------- |
| claude | `claude -p --output-format json --json-schema … --no-session-persistence --strict-mcp-config --setting-sources project` | `--tools Read,Grep,Glob`    | `structured_output`，由 Claude Code 按 schema 校验 | 按专家的档次：`haiku`、`sonnet`、`opus` |
| codex  | `codex exec --json --ephemeral --skip-git-repo-check -`                                                                 | `--sandbox read-only`       | 最后一条 `agent_message`                           | Codex 的默认模型                        |
| pi     | `pi --mode json --no-session`                                                                                           | `--tools read,grep,find,ls` | 最后一条助手消息                                   | pi 的默认模型                           |

每个工具都用它自己已有的登录。和 revmux 一样，eyeful 会从 Claude Code 的环境里去掉 `ANTHROPIC_API_KEY`，免得为别的程序设置的 key 让审查变成按 API 计费；`--setting-sources project` 让用户自己的 Claude Code 设置、hook 和 MCP 服务不进入审查。`connect` 以及每次审查开始前，都会检查工具是否在 `PATH` 上并读取它的版本。是否已登录要到第一次调用才知道，这时报错会说明怎样登录，比如运行 `claude` 再执行 `/login`。

token 数，以及工具报告了的费用（Claude Code 和 pi），都取自每次调用的输出，计入审查的预算。用量计入用户自己在该工具上的订阅或 key，和平时使用它一样。

## 本地模式的风险 {#local-risks}

审查哪些代码由用户自己选，但 eyeful 并不因此就完全信任它：

| 风险                                   | 防护                                                                           |
| -------------------------------------- | ------------------------------------------------------------------------------ |
| `.eyeful/config.yml` 里写了有害的命令  | 命令要用户确认后才运行，命令有任何改动都要重新确认                             |
| eyeful 准备改动时触发了仓库的 hook     | eyeful 执行的 git 命令都关闭了 hook                                            |
| 代码里的内容诱导 agent                 | agent 的命令行工具只带只读工具运行（`--tools`、`--sandbox read-only`）         |
| agent 给出的测试编号或修改是精心构造的 | 测试编号作为一个加了引号的参数传给 `test_one`；修改只能落在仓库内、`.git` 之外 |
| 项目命令造成破坏                       | 命令只在 eyeful 的快照检出里运行，从不在你的工作区里运行                       |
| 仓库自带一份“已确认”记录               | 确认记录按仓库路径保存在用户配置目录里，从不放在仓库里                         |
| 超时的命令留下仍在运行的进程           | eyeful 结束命令的整个进程组，而不只是它的 shell                                |
| 审查期间工作区被改动                   | 快照先固定下来；agent 可能读到了更新的代码时，eyeful 会提示                    |
| 分支是别人的，比如检出到本地的 PR      | 计划中：eyeful 给出提示，建议改用云端审查                                      |

确认过的命令以用户自己的权限运行，和用户亲手运行一样，所以本地不再另加沙箱。别人的代码应该放到云端沙箱里审查（见[云端运行](CLOUD.md#the-sandbox)）。
