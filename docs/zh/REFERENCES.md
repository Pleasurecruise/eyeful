# 参考资料

eyeful 审查时依据的标准，以及它借鉴的两个项目。

## 软件质量

- ISO/IEC 25010:2023，_Product quality model_。专家按它的质量特性划分（见[名册](EXPERTS.md#the-roster)）。

## 代码审查 {#code-review}

- Google Engineering Practices：[The Standard of Code Review](https://google.github.io/eng-practices/review/reviewer/standard.html)（评论何时阻塞，见[评审标准](REVIEW.md#review-standard)）、[What to look for](https://google.github.io/eng-practices/review/reviewer/looking-for.html)（专家问的问题；每一行都要读）和 [How to write comments](https://google.github.io/eng-practices/review/reviewer/comments.html)。
- [Conventional Comments](https://conventionalcomments.org)。每条评论的格式，以及标签和修饰词的名字（见[评论格式](REPORT.md#comment-format)）。它是社区约定，只给标签命名，不论证分类。
- M. V. Mäntylä 和 C. Lassenius，[“What Types of Defects Are Really Discovered in Code Reviews?”](https://doi.org/10.1109/TSE.2008.71)，_IEEE TSE_，35(3)，2009。评审发现分为功能性缺陷和可演化性缺陷：`note` 与其他严重度之间的那条线（见[评论格式](REPORT.md#comment-format)）。
- OASIS，[SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)。发现的保存格式，以及给发现定级的两套刻度：`kind`（§3.27.9）和 `level`（§3.27.10）（见[类别与严重度](VERIFICATION.md#kind-and-severity)）。
- Tenny，[Trust Doesn't Live in the Code Review](https://raft.build/zh-cn/resources/blog/trust-doesnt-live-in-the-code-review/)，Raft，2026。审查是一个事件，信任是一种状态；一项检查只有被看到失败过才算数（见[局限](overview.md#limits)、[多轮审查](REPORT.md#rounds)、[tests](EXPERTS.md#tests)）。

## 设计与可维护性

- T. J. McCabe，“A Complexity Measure”，_IEEE TSE_，1976。圈复杂度。
- W. Stevens、G. Myers 和 L. Constantine，“Structured Design”，_IBM Systems Journal_，1974。耦合与内聚。
- R. C. Martin，_Design Principles and Design Patterns_，2000。SOLID。
- M. Nygard，[Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)，2011。

## 正确性与安全

- [SEI CERT Coding Standards](https://wiki.sei.cmu.edu/confluence/display/seccode)。
- [CWE](https://cwe.mitre.org) 和 [CWE Top 25](https://cwe.mitre.org/top25/)。每条正确性和安全发现的弱点类别。
- [OWASP Top 10](https://owasp.org/Top10/) 和 [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/)。
- [OSV](https://osv.dev) 和 [CVE](https://www.cve.org)。依赖里的已知漏洞。

## 测试 {#testing}

- E. T. Barr、M. Harman、P. McMinn、M. Shahbaz 和 S. Yoo，“The Oracle Problem in Software Testing: A Survey”，_IEEE TSE_，41(5)，2015。specified、derived、implicit 和 human 四种测试预言（见[判定依据](VERIFICATION.md#oracle)）。
- ISO/IEC/IEEE 29148:2018，_Requirements engineering_。inspection、analysis、demonstration 和 test（见[证据](VERIFICATION.md#evidence-kinds)）。
- C. E. Jimenez 等，“SWE-bench: Can Language Models Resolve Real-World GitHub Issues?”，ICLR 2024。fail-to-pass、pass-to-pass，以及 `PASSED`、`FAILED`、`ERROR` 三种状态（见[验证器](VERIFICATION.md#the-verifier)）。
- R. Just、D. Jalali 和 M. D. Ernst，“Defects4J”，ISSTA 2014。缺陷与引入它的改动绑定：`introduced` 关卡。
- C. Zhang 等，“SoK: DARPA's AI Cyber Challenge (AIxCC): Competition Design, Architectures, and Lessons Learned”，arXiv:2602.07666，2026。先证明漏洞存在，再用复现和功能测试验证补丁：正是关卡支撑的两个结论。它是先例，不是关卡的来源。
- N. Alshahwan 等，“Automated Unit Test Improvement using Large Language Models at Meta”，FSE Companion 2024。生成的测试只有能构建、且五次运行结果一致才保留：`runs` 和 `not_flaky` 关卡。
- ISO/IEC/IEEE 29119，_Software testing_。测试设计技术。
- A. van Deursen、L. Moonen、A. van den Bergh 和 G. Kok，“Refactoring Test Code”，XP 2001。测试坏味道。
- Y. Jia 和 M. Harman，“An Analysis and Survey of the Development of Mutation Testing”，_IEEE TSE_，2011。
- Martian，“Code Review Bench”，2026，<https://github.com/withmartian/code-review-benchmark>。离线数据集，以及 [`match.py`](EVALUATION.md#tooling) 运行的裁判。

## 可用性与无障碍

- W3C，[WCAG 2.2](https://www.w3.org/TR/WCAG22/) 和 [WAI-ARIA Authoring Practices Guide](https://www.w3.org/WAI/ARIA/apg/)。
- J. Nielsen，[10 Usability Heuristics for User Interface Design](https://www.nngroup.com/articles/ten-usability-heuristics/)。
- ISO 9241-110:2020，_Interaction principles_。
- [RFC 9457](https://www.rfc-editor.org/rfc/rfc9457)，_Problem Details for HTTP APIs_，以及 [Semantic Versioning 2.0.0](https://semver.org)。

## 可读性与文档

- [Effective Go](https://go.dev/doc/effective_go)、[PEP 8](https://peps.python.org/pep-0008/) 和 [Google 风格指南](https://google.github.io/styleguide/)，在项目没有自己的规范时使用。
- [Diátaxis](https://diataxis.fr)。文档的四种类型。

## 项目

来自 [antfu/pulls.review](https://github.com/antfu/pulls.review)（MIT）。eyeful 运行 `@pulls.review/core` 给改动分组（见[计划](PLANNER.md#the-plan)），并在[终端](REPORT.md#terminal)里先展示它的摘要。另外借鉴了三个想法：

| 想法                               | 用在哪里                     |
| ---------------------------------- | ---------------------------- |
| 在任何模型运行之前先用规则分类文件 | [分拣](PLANNER.md#triage)    |
| 通过索引的副本读取工作区           | [快照](LOCAL.md#snapshot)    |
| 大型改动一次审查一个范围           | [大型改动](PLANNER.md#large) |

来自 [umputun/revmux](https://github.com/umputun/revmux)：

| 想法                                     | 用在哪里                            |
| ---------------------------------------- | ----------------------------------- |
| 固定的阶段顺序                           | [各个阶段](REVIEW.md#stages)        |
| 合并两行以内的发现并统计来源             | [排序报告](REPORT.md#ranked-report) |
| 某一步失败时带着较少的结果继续           | [各个阶段](REVIEW.md#stages)        |
| 把上一轮的发现传给下一轮                 | [多轮审查](REPORT.md#rounds)        |
| 把一轮的输入和输出放在一起               | `snapshot.json` 和 `change.diff`    |
| 每个审查者一个进程                       | 每个[专家](EXPERTS.md)运行一次      |
| 限制同时运行的审查者数量和每次调用的时间 | [Agent](LOCAL.md#agents)            |

## 与类似工具的比较 {#compared}

|              | Claude Code ultrareview | revmux                          | eyeful                                   |
| ------------ | ----------------------- | ------------------------------- | ---------------------------------------- |
| 谁挑选审查者 | 未公开                  | 调用者选择一个 profile          | 规划器给改动分组，Go 按规则挑选          |
| 审查什么     | Bug                     | profile 的各个视角覆盖的内容    | 设计、功能、安全、测试、可用性、可读性   |
| 怎样验证发现 | 在云端沙箱里复现        | 另一个 agent 阅读代码           | 验证器运行测试，其余标为 `review`        |
| 反馈         | 发现，不带修复          | JSON 或 Markdown，带 `fix` 文本 | 排序报告里的 Conventional Comments       |
| 在哪里运行   | Anthropic 的云          | 被审查的仓库内                  | 用户的机器，或每个 pull request 一个沙箱 |
