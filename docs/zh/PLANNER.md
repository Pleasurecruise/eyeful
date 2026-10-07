# 规划与分诊

审查的第一个阶段要做三件事：看改动是否到了可以审查的程度，把不需要审查的文件放到一边，再决定剩下的文件由哪些专家来审。这三步按成本从低到高依次进行。本页的步骤和规则都已在 `workflow` 中实现；L1 工具、读取 GitHub 上的 CI 结果、规划器的提示词和 `submit_plan` 工具属于运行时，还没有实现。

```mermaid
flowchart LR
    ci{"CI 校验"} -->|未通过| diagnose["CI agent<br/>原因和修复方法"]
    ci -->|通过| triage["分诊<br/>只用规则"]
    triage --> tools["L1 工具"]
    tools --> planner["规划器 agent"]
    planner -->|submit_plan| check{"Go 检查计划"}
    check -->|合格| run["启动专家"]
    check -->|不合格| retry["重试一次，<br/>仍不合格就用默认专家"]
```

## CI 校验 {#ci-check}

如果改动编译不过，或者项目自己的测试没通过，专家找到的问题多半和 CI 报的重复。所以这一阶段先看 CI。

在云端，eyeful 读取 head 提交对应的 GitHub Actions 运行记录。还有运行没结束时，审查先等待；全部通过，或者仓库没有配置 workflow，就继续往下走。

在本地没有 CI 记录可读，由验证器在工作目录里运行 [`.eyeful/config.yml`](CONFIGURATION.md#project) 中的 `lint` 命令和 L1 的 `typecheck`。完整的 `test` 命令只在 deep 档运行，因为有些项目跑一遍全部测试比整个审查还慢。

校验没通过时，不会启动任何专家，只启动 CI agent。它读取失败任务的日志（本地则读命令输出），对每处失败说明是哪个 workflow、job 和 step，原因是什么，怎么修。审查以 `ci_failed` 结束。CI 通过时这一步没有额外开销，不通过时只多跑一次 agent。

## 分诊 {#triage}

分诊只用规则给改动的文件分类，不调用模型。通过 glob 匹配找出锁文件、生成代码、第三方代码、二进制文件，以及 `.eyeful/config.yml` 里 `skip` 列出的路径；另有一条大小规则，把 diff 超过 512 KB 的文件放到一边。这些文件不交给专家，在反馈里列为“未审查”。参照 Google 的 [Every Line](https://google.github.io/eng-practices/review/reviewer/looking-for.html#every-line)，其余每一行都至少要交给一个专家。quick 档例外，只审查被工具标出的行（见[档位](LEVELS.md)）。

剩下的文件整理成一份清单，每个文件一行：

```text
path                      class   +lines  -lines
app/auth/session.py       code        12       3
app/auth/test_session.py  test         0       0
web/src/Login.svelte      code        40      18
package-lock.json         lock       310     120
commits: "fix session expiry at the boundary", "show the expiry on the login page"
```

分诊还会按项目用到的语言运行 L1 工具（`lint`、`typecheck`、`secret_scan`、`dependency_audit`、`openapi_diff`、`complexity`、`coverage_diff`），把结果附在清单后面。这些工具不消耗 token，规划器就根据它们的结果做判断（见[档位](LEVELS.md)）。

## 大改动 {#large}

分诊保留下来的文件改动超过 2000 行时，先拆成若干范围，再做任何规划。这借鉴了 pulls.review 一次只审一个范围的做法。拆分由 Go 只根据路径完成，不调用模型，所以同样的改动总是得到同样的范围，云端和本地都一样：

1. 文件按路径排序，先按第一级目录分组，再按前两级目录分组。两级之后仍超过 2000 行的组，按路径顺序切成每份不超过 2000 行的几部分；单个文件本身就更大时，自成一部分。
2. 相邻的组在合计不超过 2000 行的前提下合并，避免出现几乎空的范围。
3. 每个范围按顺序单独规划、单独分配专家，清单只包含这个范围的文件。规划器看到的清单从不超过一个范围，Go 也按各自的范围检查每份计划。
4. 每个范围最多花掉审查预算的平均一份。某个范围用完自己那份后不再启动 agent，审查结果为 `partial`，后面的范围照常运行。

档位对整个改动只选一次（见[档位的选择](LEVELS.md#choosing-the-level)），验证和汇总也只对所有范围的发现运行一次。报告按顺序列出这些范围，作者可以据此把改动拆成一组 stacked 改动（见[排序报告](REPORT.md#ranked-report)）；eyeful 从不创建分支或 pull request。

## 审查计划 {#the-plan}

quick 档没有规划器，由规则把每条工具结果分给对应的专家。standard 和 deep 档的计划分两步：先由 [`@pulls.review/core`](https://github.com/antfu/pulls.review) 弄清这次改动干了什么并分组，再由 Go 按规则给每组挑专家。

### 给改动分组

eyeful 直接使用 pulls.review 自己的分析，不是照着写一份。`packages/pulls` 把本地 agent 需要的 `@pulls.review/core` 函数打包成 `workflow/pulls/core.js`，Go 用纯 Go 写的 JavaScript 运行时 [moejs](https://github.com/Calcium-Ion/moejs) 运行它，所以二进制文件既不需要 Node，也不需要 cgo。规划器就是对已连接 agent 的一次调用：

1. core 解析分诊保留下来的文件的 diff（`parsePatch`），生成 pulls.review 给本地 agent 的提示词（`buildCliAgentSystemPrompt` 和 `buildAnalysisPrompt`）：文件清单和每个文件的 hunk 标题、放得下时的 diff，以及运行目录里 `change.diff` 的路径，清单不够时 agent 去读它；
2. agent 按 core 的 `Analysis`（`analysisJsonSchema`）作答：一段说明这次为什么改的概述，以及按意图分的组，每组有触及系统的哪一部分（`category`）、名称、说明、文件，需要格外小心的标 `critical`；
3. core 按自己的 schema 校验答案，并检查每个文件是否恰好在一个组里（`findCoverageIssues`）。不合格的答案连同 core 给出的原因退回给 agent 重做一次；第二次还不合格，或者连续两次调用失败，就使用该档位的默认专家，所有保留下来的文件放在一个组里，并在反馈里说明。

```json
{
	"overallSummary": "Sessions are now treated as expired at the exact expiry second, and the login page shows when a session ends.",
	"groups": [
		{
			"key": "session-expiry",
			"label": "Session expiry",
			"summary": "A session was still accepted at the second it expired.",
			"category": "security",
			"critical": true,
			"filePaths": ["app/auth/session.py"]
		},
		{
			"key": "login-page",
			"label": "Login page",
			"category": "ui",
			"filePaths": ["web/src/Login.svelte"]
		}
	]
}
```

core 标了 `critical` 的组，或者包含 `.eyeful/config.yml` 中 `risk` 路径的组，算作核心组。核心组排在前面，专家先看它们。

### 挑选专家

Go 用一张固定的表把每个组对应到专家，所以同样的分组总是得到同样的专家：

| 类别                                 | 专家                  |
| ------------------------------------ | --------------------- |
| `security`                           | correctness、security |
| `core`、`api`、`data`、`cli`         | correctness           |
| `ui`、`i18n`                         | usability             |
| `tests`                              | tests                 |
| `docs`、`examples`                   | readability           |
| `config`、`build`、`scripts`、`deps` | security              |
| `assets`、`other`                    | 无                    |
| 核心组另外加上                       | correctness、security |
| deep 档的核心组再加上                | design                |

之后每个专家只运行一次，审查分给它的所有组（见[专家](EXPERTS.md)）。Go 还会补上信号要求的专家（见[档位](LEVELS.md)），并分配预算。
