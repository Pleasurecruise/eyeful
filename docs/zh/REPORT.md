# 审查反馈

审查的最后一个阶段把所有发现和所有验证结果写成人能直接照着处理的评论。这一步不用 agent：`workflow` 合并发现，用专家自己写的内容为每条发现生成一条评论，再排序；`workflow/report` 把评论写成 Markdown 和 SARIF。

## 评论格式 {#comment-format}

所有评论都采用 [Conventional Comments](https://conventionalcomments.org) 格式：

```text
<label> [decorations]: <subject>

[discussion]
```

label 表示评论的类型，decorations 说明是否阻塞、涉及哪个方面，subject 是一句话的概括，discussion 解释原因和下一步怎么做。格式固定之后，同一条评论既能发到 GitHub，也能在终端里打印，还能交给别的 agent 解析。

eyeful 用规范里的四种标签，标签由 Go 按规则决定：

| 标签         | 规范里的含义                 | eyeful 把它给                                       |
| ------------ | ---------------------------- | --------------------------------------------------- |
| `issue`      | 具体的问题，最好配上建议     | 类别为 `fail` 的发现                                |
| `question`   | 审查者不确定的潜在问题       | 证据为 `test` 或 `analysis`、但还不是 `fail` 的发现 |
| `suggestion` | 改进建议，写明改什么、为什么 | 只依靠 `inspection` 的发现                          |
| `nitpick`    | 琐碎的、偏好性的请求，不阻塞 | `note` 的发现：不改变任何行为，比如命名或措辞       |

规范里另外五种标签（`praise`、`todo`、`thought`、`chore`、`note`）没有用：专家交上来的内容里没有任何东西能区分它们。

### 为什么不用 agent 来写评论 {#no-summarizer}

原先每条评论都由一个汇总 agent 来写：它读合并后的发现，从九种标签里选一个，再改写 subject 和 discussion。它被删掉了，原因有四个：

- 它选的标签已经不决定任何事情。Go 本来就会从两头改掉它：只有 `fail` 的结果才允许 `issue`，`note` 的发现强制为 `nitpick`；排序又改成了以证据和多专家一致为先，标签只排在第四位。
- 它没法核对自己写的东西。它没有读文件的工具，看到的只有发现，所以它写的只能是把专家的话换个说法，或者说偏。
- 生成评论有两条路。agent 出错或答案被拒收时，评论改由规则生成，所以规则本来就必须存在，而同一次审查可能走任何一条路。
- 每次审查它都要多花一次模型调用：大约 2% 的 token、多等一个 agent 的时间，以及多一个可能失败的环节。

这样做放弃的东西是实在的，而且还没有测过。几个专家共同支持的一条评论，现在只带着其中一个专家的原话，而那个 agent 可以把它们合在一起；措辞是专家自己的，未经整理；另外少了五种标签。改动之后还没有任何一次审查被评过分，所以评论是否读起来更差、是否更难对上基准里的发现，目前不知道。

decorations 同样按规范使用。`blocking` 和 `non-blocking` 表示合并前是否必须解决；规范里的第三个 `if-minor` 没有使用，因为专家交上来的内容里没有任何信息说明修复有多大。只有 `issue`，也就是说明存在 bug、漏洞或回归的 `fail` 结果，才标为 `blocking`（见[评审标准](REVIEW.md#review-standard)），其余都是 `non-blocking`。

另有一些 decorations 表示所属方面：`security`、`correctness`、`design`、`tests`、`ux`、`readability`。按照 Google 的[评论建议](https://google.github.io/eng-practices/review/reviewer/comments.html)，评论只针对代码、不针对作者，每条都要说明理由。评论的 subject 和 discussion 就是专家自己写的，取自它背后最严重的那条发现。

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

其余评论都放进一份报告，作为一条 pull request 评论发布，同时保存为 Markdown 和 SARIF。先合并发现：不同专家在同一文件中相距两行以内的发现合成一条，并统计有几个不同的专家报告了它。同一个专家的两条发现无论相距多近都是两个问题，各自保留一条评论。同一个模型家族的专家往往犯同样的错，所以不同模型家族之间的一致比同一家族内的一致更有分量。合并后按以下顺序排列：

1. 检查之后还站得住的排在前面：先是裁判 agent 没有质疑的评论，再是被它质疑的，最后是 nitpick；
2. 类别：先 `fail`，再 `review`，最后 `open`（见[证据与验证](VERIFICATION.md#kind-and-severity)）；
3. 报告这条发现的专家数量；
4. 标签：`issue`、`question`、`suggestion`、`nitpick`；
5. 计划里的改动分组，然后是文件和行号。

证据和多个专家的一致排在标签前面，因为它们是测出来的，而标签是从它们推出来的：在 ReviewBench 的测试 pull request 上，两个专家报告的评论匹配上已知发现的比例是单个专家的三倍，被裁判 agent 质疑的评论没有一条匹配上（见[评测](EVALUATION.md#reviewbench-pilot)）。`issue` 仍然排在最前，因为只有 `fail` 的结果才能成为 `issue`。

有两类评论折叠到最后的区块里，每条一行：被裁判 agent 质疑的评论（附上它的异议），以及 `nitpick`。一条评论背后的发现严重度全是 `note` 时，它就是 `nitpick`。报告不会漏掉任何一条；折叠只决定读的人先看到什么。报告最后列出：

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

`eyeful review` 展示发现的方式参照 Claude Code 的 ultrareview。开头是数量统计，然后像 pulls.review 那样，在任何评论之前先说明这次改动干了什么：规划器写的概述，以及各分组的表格，核心组在前，每组列出类别、文件为什么改和文件列表。随后列出在基础提交上通过、在改动上失败的项目测试（见[被改动弄坏的测试](VERIFICATION.md#regressions)）。接着是 "No blocking issues"（没有阻塞问题时），或者按排序报告的顺序列出的发现表格，其中不含 nitpick：

```text
| Severity     | File:Line            | Issue                                     |
| ------------ | -------------------- | ----------------------------------------- |
| 🔴 error     | `app/session.py:11`  | Session still valid at exactly expires_at |
| 🟠 warning   | `web/src/Login.svelte:40` | The expiry is shown in UTC            |
```

严重度是专家给的，用 SARIF 的级别：🔴 error、🟠 warning 或 🟡 note。之后每个发现有一个标题，写明严重度、所属方面和类别：eyeful 亲眼看到它的复现失败、且判定依据不是 agent 自己时为 `fail`，否则为 `review`（见[类别与严重度](VERIFICATION.md#kind-and-severity)）。第一行按类别统计发现的数量。被裁判 agent 驳回的发现（见[裁判 agent](VERIFICATION.md#the-judge)）没有标题，折叠在“Disputed”下面。标题下依次是一句话的概括、失败场景（触发问题的输入以及会出什么错）、折叠起来的 "Why this was flagged"（推理过程、eyeful 怎样验证的、复现测试），以及以 diff 形式给出的已验证修复。最后是折叠起来的 nitpick、分诊时放到一边的文件和不自信清单。

每次运行都会把同样的发现保存为 `findings.json`，字段名沿用 Claude Code 发现列表的名字（`file`、`line`、`related`、`summary`、`short_summary`、`failure_scenario`、`category`），另加 SARIF 的 `severity` 和 `kind`；同时保存 `review.md`、`results.sarif` 和完整的 `result.json`（见[本地运行](LOCAL.md)）。

各种输出都是接在工作流末端的 sink（见[整体架构](ARCHITECTURE.md#data-flow)），新增一种输出就是新增一个 sink。

## 多轮审查 {#rounds}

一次改动通常要经过几轮审查和修改。一次审查是一个事件：它对一个 diff 给出结论，然后就结束了。两轮之间人需要的是一种状态，也就是哪些问题还在；eyeful 用它亲眼看到失败过的复现测试来保存这个状态，而不是靠记住 agent 说过什么。

复审是对同一内容发起的新审查。在本地，同一内容指 `.eyeful/runs` 里最近的一次较早运行，它对比的是同一个基础提交，并且它的 head 就是这次所审的 head 或者它的祖先；审查未提交的改动时，基础提交相同即可。另一处改动即使正好从上一处结束的地方开始，基础提交也不同，不会带过任何东西。用 `execution` 时，在任何专家开始之前，验证器先把上一轮看到失败的每个复现测试各重跑五次，这一步不调用模型：

| 复现测试现在                                | 这条发现                                                                                 |
| ------------------------------------------- | ---------------------------------------------------------------------------------------- |
| 每次都通过                                  | 已解决。报告在“Resolved since the last review”下列出它和它的测试，不再有评论             |
| 带着 `fails` 文本失败，而在基础提交上不这样 | 仍未解决。它带着同一个测试和判定依据进入这一轮，专家会被告知它已经被发现                 |
| 在基础提交上以同样方式失败                  | 原来就有：问题已经合并进去，不再是这次改动引入的。照常报告，但不阻塞                     |
| 不稳定、以别的方式失败，或者无法运行        | 作为未验证的评论再次展示，并列入不确定性清单；测试路径上现在已有文件是无法运行的原因之一 |

一条发现只有在验证器眼前从红变绿，才算修好。上一轮用这个版本已经没有的刻度（它不认识的判定依据或严重度）评定的发现不会带过来，上一轮没有复现成功的发现也不会：专家重新读这次改动，问题还在就会再报一次。用 `read_only` 或 `none` 时不运行任何东西，所以也不带任何发现。把上一轮未被证明的发现再交给裁判 agent 读一遍，还在计划中。
