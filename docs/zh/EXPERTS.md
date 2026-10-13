# 专家

专家是执行审查的 agent。每个专家覆盖 ISO/IEC 25010 质量模型的一部分，并有自己的 skill。Go 为计划里的每个组挑选需要的专家（见[计划](PLANNER.md#the-plan)），每个专家对自己的所有组运行一次，最多四个同时运行。这里提到的标准都列在[参考资料](REFERENCES.md)里。

## 名册 {#the-roster}

| 专家        | ISO/IEC 25010 特性         | 模型档位 |
| ----------- | -------------------------- | -------- |
| design      | 可维护性：模块化、可修改性 | strong   |
| correctness | 功能适合性、可靠性         | strong   |
| security    | 安全性                     | strong   |
| tests       | 可维护性：可测试性         | mid      |
| usability   | 交互能力                   | mid      |
| readability | 可维护性：可分析性         | cheap    |
| skeptic     | 功能适合性、可靠性         | strong   |
| generalist  | 以上全部，独自完成         | strong   |

- skeptic 审查和 correctness 相同的组，但看不到计划。
- generalist 只在被点名时运行，比如 `eyeful review generalist`，并拥有全部 skill。它是[评估](EVALUATION.md)里的单 agent 基线。
- 还没有专家覆盖性能（见[路线图](ROADMAP.md)）。
- 档位通过[供应商路由](cloud/SCHEDULING.md#agent-runs-over-providers)选择模型。

## 提示词、skill 与工具 {#skills-and-tools}

**提示词。** 专家的提示词是 `workflow/prompts/experts/` 下它那个文件的正文。规划器、修复 agent、复现 agent、裁判和 CI agent 的提示词在 `workflow/prompts/roles/` 下。标准不写在提示词里。

**Skill。** skill 是一个带 `SKILL.md` 的文件夹，采用 [skills.sh](https://skills.sh) 的 Agent Skills 格式。eyeful 不自己写 skill：它把安装量大的 skill 原样放在 `workflow/prompts/skills/` 下，连同 `skills-lock.json` 一起嵌入二进制。

- skill 的文件和锁文件里的哈希对不上时，eyeful 拒绝启动。`mise run skills` 会重新安装。
- 语言类 skill 只在专家的组里有该语言的文件时才提供。
- 一个 skill 只属于专家组里的一个专家。generalist 拥有全部。

| Skill                         | 来源                                | 专家，以及提供给哪些文件                     |
| ----------------------------- | ----------------------------------- | -------------------------------------------- |
| `code-review`                 | `anthropics/knowledge-work-plugins` | correctness                                  |
| `golang-error-handling`       | `samber/cc-skills-golang`           | correctness，Go                              |
| `python-error-handling`       | `wshobson/agents`                   | correctness，Python                          |
| `typescript-advanced-types`   | `wshobson/agents`                   | correctness，JavaScript 和 TypeScript        |
| `golang-concurrency`          | `samber/cc-skills-golang`           | skeptic，Go                                  |
| `async-python-patterns`       | `wshobson/agents`                   | skeptic，Python                              |
| `security-review`             | `getsentry/skills`                  | security                                     |
| `golang-security`             | `samber/cc-skills-golang`           | security，Go                                 |
| `codebase-design`             | `mattpocock/skills`                 | design                                       |
| `golang-design-patterns`      | `samber/cc-skills-golang`           | design，Go                                   |
| `python-design-patterns`      | `wshobson/agents`                   | design，Python                               |
| `nodejs-backend-patterns`     | `wshobson/agents`                   | design，JavaScript 和 TypeScript             |
| `tdd`                         | `mattpocock/skills`                 | tests                                        |
| `golang-testing`              | `samber/cc-skills-golang`           | tests 和复现 agent，Go                       |
| `python-testing-patterns`     | `wshobson/agents`                   | tests 和复现 agent，Python                   |
| `javascript-testing-patterns` | `wshobson/agents`                   | tests 和复现 agent，JavaScript 和 TypeScript |
| `accessibility`               | `addyosmani/web-quality-skills`     | usability                                    |
| `golang-code-style`           | `samber/cc-skills-golang`           | readability，Go                              |
| `golang-naming`               | `samber/cc-skills-golang`           | readability，Go                              |
| `python-code-style`           | `wshobson/agents`                   | readability，Python                          |
| `modern-javascript-patterns`  | `wshobson/agents`                   | readability，JavaScript 和 TypeScript        |
| `git-commit`                  | `github/awesome-copilot`            | 仅 `eyeful commit`                           |

**工具。** `workflow/prompts/tools.yaml` 定义云端运行时的工具：

| 工具                                           | 给 agent 提供                | 角色                         |
| ---------------------------------------------- | ---------------------------- | ---------------------------- |
| `git_diff`                                     | 改动，单个文件或全部         | 所有角色                     |
| `git_log`                                      | 改动之前的历史               | CI agent、规划器、专家、裁判 |
| `git_blame`                                    | 某段代码最后由谁修改         | 专家、修复 agent、裁判       |
| `list_skills`、`load_skill`、`read_skill_file` | 提供给它的 skill             | 专家、复现 agent             |
| `try_repro`                                    | 在全新副本上运行一次复现测试 | 复现 agent                   |
| `submit_plan`、`submit_report`、…              | 该角色的结果                 | 每个角色一个                 |

本地审查没有工具服务器（见 [Agent](LOCAL.md#agents)）：

- diff 和提交信息是输入的一部分。
- skill 写到一个临时目录，输入里列出它们的路径。
- 补丁旁边有一个文件，记录每个改动文件最近的提交、被替换的行最后由谁修改，用来代替 `git_log` 和 `git_blame`。
- agent 以一个 JSON 块结束，代替提交工具的调用。
- `try_repro` 没有对应物：测试由验证器运行（见[验证器](VERIFICATION.md#the-verifier)）。

agent 保留自己读文件的工具。它编辑文件或运行命令的请求会被 eyeful 拒绝。下面提到的其他工具都在规划中（见[层级](LEVELS.md#planned)）。

## 专家报告什么 {#context}

专家拿到 diff、它那些组的文件、提供给它的 skill，以及计划（skeptic 除外）。它看不到其他专家的输出。在 `execution` 下它还会被告知改动弄坏了哪些测试（见[回归](VERIFICATION.md#regressions)），再次审查时还有哪些之前的发现仍然失败（见[多轮审查](REPORT.md#rounds)）。

它阅读每个改动的 hunk，报告改动引入的每个问题，不需要证明。一条发现包含：

- 文件、行和主题；
- `related`：同一问题出现的其他改动位置；
- 严重度：`error`、`warning` 或 `note`；
- 它的专家文件允许的证据类型；
- 它依据的 skill，或者 `other` 加一个类别，适用时带 CWE 编号；
- `test` 和 `analysis` 发现还要有场景：触发它的输入、实际发生什么、应该发生什么。

缺少其中任何一项、位于专家的组之外的文件上、或引用了没有提供给该专家的 skill 的发现，会被 Go 拒绝。被拒绝的报告退回一次。仍然无效的发现被丢弃并列入不确定性清单，提供给专家而它没有加载的 skill 也一样。本地审查看不到哪些 skill 被读过。

超时或失败的专家会被去掉并列为缺席，审查结果为 `partial`。

## design

问的是：改动是否放对了地方，是否和系统的其余部分相称。

- 标准：Google 关于设计和复杂度的问题、耦合与内聚、SOLID、圈复杂度、项目的架构记录。
- 规划中的工具：`complexity`、`dependency_graph`、`find_references`。
- 多数发现靠 `inspection`，会成为 `suggestion` 评论。每条都要指出依据的原则。

## correctness

问的是：改动是否做到了作者想做的事。边界、空输入和超大输入、错误路径、并发、资源泄漏。

- 标准：Google 关于功能的问题、SEI CERT、CWE。
- 规划中的工具：`typecheck`、`static_analysis`。
- 它报告能暴露错误的那次调用：输入和错误的结果。

## skeptic

覆盖和 correctness 相同的范围，但不被告知改动的目的。

- 对每一行被删除或改写的代码，找出依赖旧行为的地方。
- 对每一行新增的代码，找出作者没有预料到的输入、状态或调用顺序。
- 两个专家到达同一位置算作互相印证（见[报告](REPORT.md#ranked-report)）。

## security

问的是：改动是否引入了漏洞。

- 标准：OWASP Top 10、OWASP ASVS、CWE Top 25、SEI CERT。
- 规划中的工具：`secret_scan`、`dependency_audit`、`cve_lookup`、`sast`。
- 每条发现都带 CWE 编号。

## tests

问的是：改动里的测试是否值得保留。

- 标准：Google 关于测试的问题、ISO/IEC/IEEE 29119、van Deursen 等人的测试坏味道、变异测试。
- 规划中的工具：`coverage_diff`、`test_on_base`、`mutate`。
- 对每个新增或修改的测试，它指出被测代码里哪种错误会让测试失败。没有任何错误能让它失败的测试就是一条发现。
- 缺失的覆盖是 `suggestion`。它不替作者补测试。

## usability

从使用者的角度看改动：使用界面的人、调用 API 的开发者、阅读错误信息的运维。

- 标准：WCAG 2.2、WAI-ARIA Authoring Practices、Nielsen 启发式原则、ISO 9241-110、RFC 9457、语义化版本。
- 规划中的工具：`openapi_diff`、`a11y_check`、`e2e_run`。
- 它阅读标记和组件代码。目前没有任何东西渲染页面。

## readability

问的是：下一个工程师能否读懂这次改动。

- 标准：先看项目自己的风格规范，然后是语言的惯例（Effective Go、PEP 8、Google 风格指南），文档看 Diátaxis。
- 规划中的工具：`lint`、`similar_code`。
- 项目规范没有覆盖的点是 `nitpick`。

## 专家文件

每个专家是一个 Markdown 文件，嵌入在二进制里。`workflow/prompts/roles/expert.md` 补上所有专家共用的部分。

```markdown
---
name: security
area: security
tier: strong
evidence: [test, analysis, inspection]
skills:
  - name: security-review
  - name: golang-security
    files: ['**/*.go']
---

You are the security expert on a code review. ...
```

`area` 是它的评论带的修饰词。增加专家就是增加一个文件，是否保留由[评估](EVALUATION.md#ablations)决定。
