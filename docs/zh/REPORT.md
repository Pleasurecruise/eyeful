# 审查反馈

汇总是审查中最后一个 agent。它拿到所有发现和所有验证结果，写成人能直接照着处理的评论。合并、检查、排序和按规则排版都已在 `workflow` 中实现，`workflow/report` 再把评论写成 Markdown 和 SARIF。

## 评论格式 {#comment-format}

所有评论都采用 [Conventional Comments](https://conventionalcomments.org) 格式：

```text
<label> [decorations]: <subject>

[discussion]
```

label 表示评论的类型，decorations 说明是否阻塞、涉及哪个方面，subject 是一句话的概括，discussion 解释原因和下一步怎么做。格式固定之后，同一条评论既能发到 GitHub，也能在终端里打印，还能交给别的 agent 解析。

eyeful 用到了规范推荐的全部九种标签：

| 标签         | 规范中的含义                 | eyeful 在什么时候用                      |
| ------------ | ---------------------------- | ---------------------------------------- |
| `praise`     | 真诚的肯定                   | 专家注意到明显的改进                     |
| `nitpick`    | 出于个人偏好的小要求，不阻塞 | 项目风格指南没有涉及的风格问题           |
| `suggestion` | 改进建议，写明改什么、为什么 | 一处可以改进的地方，修法明确时附上补丁   |
| `issue`      | 具体的问题，最好附带建议     | 有强证据的问题，或违反了风格指南明文规定 |
| `todo`       | 必要但很小的改动             | 版本号、changelog 条目、漏掉的导出       |
| `question`   | 评审者不确定的潜在问题       | 专家没能证明的疑点                       |
| `thought`    | 审查时想到的点子，不阻塞     | 以后值得考虑的设计方向                   |
| `chore`      | 合入前要完成的流程性任务     | 项目流程要求的步骤，比如更新文档，附链接 |
| `note`       | 提醒读者注意的信息，不阻塞   | 背景信息，比如某个依赖换了许可证         |

decorations 同样按规范使用。`blocking` 和 `non-blocking` 表示合并前是否必须解决，`if-minor` 表示改动不大时才需要改。只有带强证据、说明存在 bug、漏洞或回归的 `issue` 才标为 `blocking`（见[评审标准](REVIEW.md#review-standard)），其余都是 `non-blocking`。

另有一些 decorations 表示所属方面：`security`、`correctness`、`design`、`tests`、`ux`、`readability`。按照 Google 的[评论建议](https://google.github.io/eng-practices/review/reviewer/comments.html)，评论只针对代码、不针对作者，每条都要说明理由。汇总根据每条发现的 SARIF 结果写出评论（见[证据与验证](VERIFICATION.md#sarif)）。

```text
issue (blocking, correctness): A session is still valid at exactly expires_at.

is_expired compares with >, so a request at the expiry second is accepted. The reproduction
test below fails on this change and passes with the suggested fix; the rest of the suite stays
green.
```

```text
nitpick (non-blocking, readability): `t` could say what it holds, such as `expires_at`.
```

## 行内评论

eyeful 以普通的 pull request review 发布评论，事件类型是 `COMMENT`，不使用 `REQUEST_CHANGES` 或 `APPROVE`。每条 `issue (blocking)` 都放在发现所在的那一行，验证过的修复以 GitHub 的 suggested change 附上，作者点一下就能应用。标签告诉作者哪些必须改，但这个 review 本身不会阻止合并。问题修完后也没有需要撤销的审查状态，最后由人来合并。

需要硬性门禁的仓库可以打开一个可选的 Check Run，默认关闭。只要还有 `issue (blocking)`，它就是失败状态，否则通过。Check Run 跟着提交走，推送新提交并重新审查后会被替换，分支保护规则也可以把它设为必需。

## 排序报告 {#ranked-report}

其余评论都放进一份报告，作为一条 pull request 评论发布，同时保存为 Markdown 和 SARIF。先合并发现：同一文件中相距两行以内的发现合成一条，并统计有几个不同的专家报告了它。同一个模型家族的专家往往犯同样的错，所以不同模型家族之间的一致比同一家族内的一致更有分量。合并后按以下顺序排列：

1. 标签：`issue`、`todo`、`chore`、`suggestion`、`question`、`thought`、`note`、`nitpick`、`praise`；
2. 证据强度（见[证据与验证](VERIFICATION.md#evidence-kinds)）；
3. 报告它的专家数量；
4. 审查计划中的改动分组，然后是文件和行号。

所有 `nitpick` 都折叠到最后的一个区块里。报告最后列出：

- 改动被拆成[范围](PLANNER.md#large)时，按顺序列出各范围的文件数、行数和发现数，每条发现也注明所属范围；
- 分诊时放到一边的文件，标为未审查；
- 不自信清单：缺席的专家、提供了却没被加载的 skill、没能复现的发现、没有运行的命令和测试套件，以及预算是否用完。

## 输出位置

| 输出          | 去处                                                   |
| ------------- | ------------------------------------------------------ |
| GitHub review | 行内评论，以 `COMMENT` review 发布（云端）             |
| GitHub 评论   | pull request 上的排序报告（云端）                      |
| Check Run     | 可选，默认关闭：还有 `issue (blocking)` 时失败（云端） |
| Markdown      | 控制台和终端；两部分都有，阻塞的在前                   |
| Findings JSON | 桌面端：差异旁和对应行上的发现                         |
| SARIF         | 其他 agent、脚本、GitHub code scanning、效果评测       |

GitHub 上的输出以及控制台的展示还在计划中。

## 终端里的输出 {#terminal}

`eyeful review` 展示发现的方式参照 Claude Code 的 ultrareview。开头是数量统计（没有阻塞问题时写 "No blocking issues"），以及按严重程度排序的表格：

```text
| Severity     | File:Line            | Issue                                     |
| ------------ | -------------------- | ----------------------------------------- |
| 🔴 Important | `app/session.py:11`  | Session still valid at exactly expires_at |
| 🟡 Nit       | `web/src/Login.svelte:40` | Name the expiry variable             |
```

阻塞的 `issue` 显示为 🔴 Important，其他标签都显示为 🟡 Nit。之后每个发现有一个标题，写明严重程度、所属方面和结论：证据强时为 CONFIRMED（比如 eyeful 亲眼看到失败、且判定依据来自外部的复现），否则为 PLAUSIBLE。标题下依次是一句话的概括、失败场景（触发问题的输入以及会出什么错）、折叠起来的 "Why this was flagged"（推理过程、eyeful 怎样验证的、复现测试），以及以 diff 形式给出的已验证修复。最后是分诊时放到一边的文件和不自信清单。

每次运行都会把同样的发现保存为 `findings.json`，字段名沿用 Claude Code 发现列表的名字（`file`、`line`、`summary`、`short_summary`、`failure_scenario`、`category`、`verdict`），另加 `severity`；同时保存 `review.md`、`results.sarif` 和完整的 `result.json`（见[本地运行](LOCAL.md)）。

各种输出都是接在工作流末端的 sink（见[整体架构](ARCHITECTURE.md#data-flow)），新增一种输出就是新增一个 sink。

## 多轮审查

一次改动通常要经过几轮审查和修改。复审是对同一内容发起的新审查，会拿到上一轮的发现，但拿不到上一轮的推理过程。专家被要求重新判断这些发现，而不是照单确认；已经修好的阻塞问题会在新一轮里标为已解决。

## 汇总失败时

汇总 agent 为每条合并后的发现写 label、subject 和 discussion，decorations 由 Go 添加：只有带强证据的 `issue` 标 `blocking`，再加上提出这条发现的各个专家所属的方面。以下情况 Go 会拒收汇总：某条合并后的发现没有恰好对应一条评论；label 不在九种之内；`issue` 没有强证据。

汇总出错或被拒收时，改用规则来排版：只按证据强度给标签，强证据记为 `issue`，其余记为 `question`，按文件分组。不会丢掉任何发现。
