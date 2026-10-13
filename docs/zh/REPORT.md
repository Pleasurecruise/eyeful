# 审查反馈

最后一个阶段把发现和验证结果变成评论。这里没有 agent：`workflow` 合并属于[同一个问题](VERIFICATION.md#one-problem)的发现，按规则打标签、排序，`workflow/report` 把它们渲染成 Markdown 和 SARIF。

## 评论格式 {#comment-format}

每条评论都遵循 [Conventional Comments](https://conventionalcomments.org)：

```text
<label> [decorations]: <subject>

[discussion]
```

Go 从四个标签里选一个：

| 标签         | 用于                                                |
| ------------ | --------------------------------------------------- |
| `issue`      | 类别为 `fail` 的发现                                |
| `question`   | 证据是 `test` 或 `analysis`、但还不是 `fail` 的发现 |
| `suggestion` | 只靠 `inspection` 的发现                            |
| `nitpick`    | `note` 发现                                         |

修饰词说明评论是否阻塞、属于哪个方面：`security`、`correctness`、`design`、`tests`、`ux`、`readability`。只有 `issue` 是 `blocking`（见[一次审查](REVIEW.md#review-standard)）。主题和讨论是专家自己的原话，取自评论背后最严重的那条发现。

```text
issue (blocking, correctness): A session is still valid at exactly expires_at.

is_expired compares with >, so a request at the expiry second is accepted. The reproduction
test below fails on this change and passes with the suggested fix.
```

## 行内评论

这是云端的规划。eyeful 以事件 `COMMENT` 提交 pull request review，从不使用 `REQUEST_CHANGES` 或 `APPROVE`。每条 `issue (blocking)` 放在它所在的行上，验证过的修复作为 GitHub suggested change 附上。想要硬性关卡的仓库可以启用 Check Run（默认关闭），只要还有 `issue (blocking)` 它就失败。

## 排序报告 {#ranked-report}

其余内容都进入一份报告。属于[同一个问题](VERIFICATION.md#one-problem)的发现会合并，评论记录有几个专家报告了它。评论按以下顺序排列：

1. 是否站得住：裁判没有异议的，然后是有异议的，最后是 nitpick；
2. 类别：`fail`、`review`、`open`（见[类别与严重度](VERIFICATION.md#kind-and-severity)）；
3. 报告它的专家数量；
4. 标签：`issue`、`question`、`suggestion`、`nitpick`；
5. 计划里的分组，然后是文件和行。

有异议的评论和 nitpick 折叠在末尾的收起区域里。没有任何内容被丢掉。报告的结尾是：

- 被拆分的改动的各个[范围](PLANNER.md#large)，以及它们的文件和发现；
- 分拣时放到一边的文件；
- 不确定性清单：缺席的专家、没有加载的 skill、没能复现的发现、没有运行的命令，以及预算是否用完。

## 输出位置

| 输出           | 给谁                                         |
| -------------- | -------------------------------------------- |
| Markdown       | 终端和控制台                                 |
| Findings JSON  | 桌面应用，显示在 diff 旁边                   |
| SARIF          | 其他 agent、脚本、GitHub code scanning、评估 |
| GitHub review  | 行内评论（规划中）                           |
| GitHub comment | 排序报告（规划中）                           |
| Check Run      | 可选的关卡（规划中）                         |

输出是接在工作流末尾的 sink（见[架构](ARCHITECTURE.md#data-flow)）。

## 终端里的输出 {#terminal}

`eyeful review` 依次打印：

1. 按类别统计的发现数量；
2. 改动做了什么：规划器的摘要和分组，核心组在前；
3. 被改动弄坏的测试（见[回归](VERIFICATION.md#regressions)）；
4. “No blocking issues”，或者按报告顺序排列的发现表：

```text
| Severity     | File:Line                 | Issue                                     |
| ------------ | ------------------------- | ----------------------------------------- |
| 🔴 error     | `app/session.py:11`       | Session still valid at exactly expires_at |
| 🟠 warning   | `web/src/Login.svelte:40` | The expiry is shown in UTC                |
```

5. 每条发现：严重度、方面和类别，一句话摘要，失败场景，推理过程，验证方式，复现测试，以及 diff 形式的修复；
6. 有异议的评论、nitpick、被放到一边的文件和不确定性清单。

每次运行都会保存 `review.md`、`findings.json`、`results.sarif` 和 `result.json`（见[本地运行](LOCAL.md)）。

## 多轮审查 {#rounds}

再次审查是对同一对象的一次新审查：在本地，就是 `.eyeful/runs` 里基准提交相同、且 head 是当前 head 或其祖先的最近一次运行。在 `execution` 下，专家开始之前，验证器把上一次审查看到失败的每个复现各运行五次，不调用模型：

| 复现现在的情况                            | 这条发现                                         |
| ----------------------------------------- | ------------------------------------------------ |
| 每次都通过                                | 已解决，列在 “Resolved since the last review” 下 |
| 以 `fails` 文本失败，且在基准提交上不失败 | 仍未解决；连同测试一起带入本次审查，并告知专家   |
| 在基准提交上以同样方式失败                | 原本就有；会报告，永不阻塞                       |
| 不稳定、以别的方式失败或无法运行          | 作为未验证的评论展示，并列入不确定性清单         |

上一次审查没能复现的发现不会带过来：专家重新阅读改动。在 `read_only` 或 `none` 下什么都不带。
