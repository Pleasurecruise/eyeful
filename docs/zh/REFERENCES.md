# 参考文献

本页列出 eyeful 审查时依据的标准，以及 eyeful 所依托的两个项目。其他页面需要时链接到这里。

## 软件质量

- ISO/IEC 25010:2023，_Product quality model_（产品质量模型）。专家按它的质量特性划分，每个专家负责其中一项或几项（见[专家名单](EXPERTS.md#the-roster)）。

## 代码评审

- Google Engineering Practices，[The Standard of Code Review](https://google.github.io/eng-practices/review/reviewer/standard.html)。用来决定意见什么时候阻塞：改动让代码质量变差时才阻塞；有数据的意见优先于个人看法；风格以风格指南为准（见[评审标准](REVIEW.md#review-standard)）。
- Google Engineering Practices，[What to look for in a code review](https://google.github.io/eng-practices/review/reviewer/looking-for.html)。各专家要问的问题，以及每一行都要有人看（见[规划与分诊](PLANNER.md)）。
- Google Engineering Practices，[How to write code review comments](https://google.github.io/eng-practices/review/reviewer/comments.html)。评论针对代码、不针对人，说明理由，标明严重程度（见[审查反馈](REPORT.md)）。
- [Conventional Comments](https://conventionalcomments.org)。评论的格式、标签和修饰词（见[评论格式](REPORT.md#comment-format)）。
- OASIS，[SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)，静态分析结果交换格式。发现及其证据以这种格式保存（见[用 SARIF 保存发现](VERIFICATION.md#sarif)）。

## 设计与可维护性

- T. J. McCabe，"A Complexity Measure"，_IEEE Transactions on Software Engineering_，1976。圈复杂度，design 专家的 `complexity` 工具测量的就是它。
- W. Stevens、G. Myers、L. Constantine，"Structured Design"，_IBM Systems Journal_，1974。耦合与内聚。
- R. C. Martin，_Design Principles and Design Patterns_，2000。SOLID 原则。
- M. Nygard，[Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)，2011。架构决策记录，项目有的话会读。

## 正确性与安全

- [SEI CERT Coding Standards](https://wiki.sei.cmu.edu/confluence/display/seccode)，用于有对应标准的语言。
- [CWE](https://cwe.mitre.org) 和 [CWE Top 25](https://cwe.mitre.org/top25/)。正确性和安全发现的缺陷分类。
- [OWASP Top 10](https://owasp.org/Top10/)。security 专家检查 Web 应用风险时用的清单。
- [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/)。分级的安全要求，eyeful 自己的登录实现也按它来做（见[登录与凭据](AUTH.md)）。
- [OSV](https://osv.dev) 和 [CVE](https://www.cve.org)。依赖的已知漏洞。

## 测试

- ISO/IEC/IEEE 29119，_Software testing_（软件测试）。测试设计技术和术语。
- A. van Deursen、L. Moonen、A. van den Bergh、G. Kok，"Refactoring Test Code"，XP 2001。测试坏味道。
- Y. Jia、M. Harman，"An Analysis and Survey of the Development of Mutation Testing"，_IEEE Transactions on Software Engineering_，2011。变异测试，用来检查代码出错时测试会不会失败。

## 可用性与无障碍

- W3C，[WCAG 2.2](https://www.w3.org/TR/WCAG22/)。无障碍成功准则。
- W3C，[WAI-ARIA Authoring Practices Guide](https://www.w3.org/WAI/ARIA/apg/)。常见控件的无障碍实现方式。
- J. Nielsen，[10 Usability Heuristics for User Interface Design](https://www.nngroup.com/articles/ten-usability-heuristics/)。
- ISO 9241-110:2020，_Interaction principles_（交互原则）。适合任务、自我描述、符合预期、易于学习、可控、容错、吸引用户。
- [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457)，_Problem Details for HTTP APIs_，以及 [Semantic Versioning 2.0.0](https://semver.org)。错误响应的格式和 API 的破坏性变更。

## 可读性与文档

- [Effective Go](https://go.dev/doc/effective_go)、[PEP 8](https://peps.python.org/pep-0008/) 和 [Google 风格指南](https://google.github.io/styleguide/)。项目没有自己的风格指南时使用的语言约定。
- [Diátaxis](https://diataxis.fr)。文档的四种类型，用来检查文档有没有跟着行为一起改。

## 借鉴的项目

eyeful 建立在两个项目之上：直接运行其中的 `@pulls.review/core`，并借鉴了两者的思路。

[antfu/pulls.review](https://github.com/antfu/pulls.review)（MIT）用在规划和报告两处。eyeful 直接运行 `@pulls.review/core` 来理解改动并分组（见[审查计划](PLANNER.md#the-plan)），下面这些来自它的代码：

| 内容                               | 在 eyeful 中的位置                                                   |
| ---------------------------------- | -------------------------------------------------------------------- |
| 先读完整个改动，再拆分             | 规划器的提示词就是 core 给本地 agent 的提示词                        |
| 按意图分组，标明触及系统的哪一部分 | core 的 `Analysis`：带 `category` 和 `critical` 的分组，由 core 校验 |
| 在评论之前先说明改动干了什么       | [终端输出](REPORT.md#terminal)开头 core 给出的概述和分组             |

下面这些是借鉴思路、由 eyeful 自己实现的：

| 思路                          | 在 eyeful 中的位置                                                     |
| ----------------------------- | ---------------------------------------------------------------------- |
| 调用模型之前，先用规则分诊    | [分诊](PLANNER.md#triage)：用 glob 规则把锁文件、生成代码等放到一边    |
| 通过一份 index 副本读取工作区 | 未提交改动的[快照](LOCAL.md#snapshot)                                  |
| 一次只审一个范围              | [大改动](PLANNER.md#large)：由 Go 拆分范围，超过 512 KB 的文件放到一边 |

[umputun/revmux](https://github.com/umputun/revmux) 的思路用在流程和反馈上：

| 思路                                       | 在 eyeful 中的位置                                                 |
| ------------------------------------------ | ------------------------------------------------------------------ |
| 固定的几个阶段                             | [各个阶段](REVIEW.md#stages)                                       |
| 同一文件相距两行以内的发现合并，按来源计数 | [排序报告](REPORT.md)中的合并和佐证                                |
| 某一步出错时降级继续，不中止整个审查       | [各个阶段](REVIEW.md#stages)的出错处理                             |
| 下一轮带上上一轮的发现                     | [多轮审查](REPORT.md)                                              |
| 把一轮的输入和输出放在一起                 | [运行目录](LOCAL.md#snapshot)里的 `snapshot.json` 和 `change.diff` |
| 每个审查者一个进程，不管它负责多少内容     | 每个[专家](EXPERTS.md)只运行一次，覆盖它的所有分组                 |
| 限制同时运行的审查者，每个进程有时间上限   | 同时最多四个；每次调用 2 分钟无输出或总共 20 分钟即停止            |

revmux 的验证阶段只读代码、不运行，[效果评测](EVALUATION.md)把它作为外部参照。

## 与同类工具的比较 {#compared}

|            | Claude Code ultrareview | revmux                                 | eyeful                                                |
| ---------- | ----------------------- | -------------------------------------- | ----------------------------------------------------- |
| 谁挑审查者 | 未公开                  | 调用方选 profile，阵容固定             | 规划器按改动挑选，Go 按档位检查                       |
| 审查什么   | bug                     | profile 里各个 lens 覆盖的内容         | 设计、功能、安全、测试、可用性、可读性                |
| 怎么验证   | 在云端沙箱中复现        | 另一个 agent 读代码后给出结论          | 验证器运行复现测试，较弱的发现标明证据强度            |
| 反馈形式   | 发现列表，不附修复      | JSON 或 Markdown，每条发现附一段 `fix` | Conventional Comments，阻塞的放在行内，其余进排序报告 |
| 在哪里运行 | Anthropic 的云端        | 被审查的仓库里，默认信任这个仓库       | 用户的电脑，或不含凭据的云端沙箱                      |
