# 专家

专家是负责审查的 subagent。每个专家负责 ISO/IEC 25010 产品质量模型中的一部分，依据这部分的公开标准审查，并有自己的检查清单（skill）和工具。每组需要哪些专家由 Go 按分组的类别决定（见[审查计划](PLANNER.md#the-plan)），看得多仔细、每条发现有多严重由专家自己决定（见[档位](LEVELS.md#choosing-the-level)）。`workflow` 让每个选中的专家只运行一次，覆盖分给它的所有组，同时最多运行四个，并检查它们的报告。七个专家文件都已写好。各专家小节里提到的工具，除了[提示词、skill 和工具](#skills-and-tools)里列出的，都还在计划中。提到的标准都列在[参考文献](REFERENCES.md)里。

## 专家名单 {#the-roster}

| 专家        | 对应的 ISO/IEC 25010 质量特性 | 模型档次 |
| ----------- | ----------------------------- | -------- |
| design      | 可维护性：模块化、可修改性    | 强       |
| correctness | 功能适合性、可靠性            | 强       |
| security    | 安全性                        | 强       |
| tests       | 可维护性：可测试性            | 中       |
| usability   | 交互能力                      | 中       |
| readability | 可维护性：可分析性            | 便宜     |
| skeptic     | 功能适合性、可靠性            | 强       |
| generalist  | 以上全部，独自承担            | 强       |

skeptic 和 correctness 专家审查同样的分组，但它是盲审：只拿到文件，拿不到计划，所以规划器对改动的一次错误解读不会同时带偏所有审查者。它追问改动删掉的每一行原来被谁依赖，新增的每一行没有料到哪种输入。规划器从不选 generalist。只有点名时它才运行，比如 `eyeful review generalist`，这时它一个人带着其他专家的全部 skill 审查整个改动：它就是[效果评测](EVALUATION.md)里用来和专家组比较的单 agent 基线。实验的问题只涉及 correctness 和 security 的发现，因为只有它们能够复现；其余四个专家在本地审查中使用。性能效率目前还没有对应的专家（见[版本路线图](ROADMAP.md)）。模型档次通过[模型路由](SCHEDULING.md#agent-runs-over-providers)对应到一组具体模型。

## 提示词、skill 和工具 {#skills-and-tools}

eyeful 启动的每个 agent 都会拿到一段提示词、一组 skill 和一组工具。提示词说明这个 agent 是谁，再给几句简单的指导：专家的提示词就是专家文件的正文；规划器、修复 agent、复现 agent、评判 agent、汇总 agent 和 CI agent 的提示词在 `workflow/prompts/roles/` 下。标准和检查清单不放进提示词。

skill 是一个带 `SKILL.md` 的文件夹，格式是 [skills.sh](https://skills.sh) 分发的 Agent Skills 格式。eyeful 不自己写 skill，而是从 skills.sh 上挑安装量高的，原样放进 `workflow/prompts/skills/`，连同 skills 命令行工具生成的 `skills-lock.json` 一起内嵌进二进制文件。skill 的文件和锁文件里的哈希对不上时，eyeful 拒绝启动；`mise run skills` 会按锁文件重新安装。专家文件列出自己用哪些 skill；语言相关的 skill 还会写明适用的文件，只有专家负责的分组里有这类文件时才提供给它。角色文件用同样的方式列出 skill。复现 agent 的是 `golang-testing`、`python-testing-patterns` 和 `javascript-testing-patterns`：提供给它的是发现所在文件那门语言的那一个，所以在旁边的测试没有先例可循的地方，它写的测试遵循这门语言的惯例。

| skill                                            | 来源                                | skills.sh 安装量 | 专家                                |
| ------------------------------------------------ | ----------------------------------- | ---------------- | ----------------------------------- |
| `code-review`                                    | `anthropics/knowledge-work-plugins` | 9.7K             | correctness、skeptic                |
| `tdd`                                            | `mattpocock/skills`                 | 1M               | tests                               |
| `golang-error-handling`、`golang-concurrency`    | `samber/cc-skills-golang`           | 各约 42K         | correctness、skeptic，Go 文件       |
| `python-error-handling`、`async-python-patterns` | `wshobson/agents`                   | 13.3K、17.1K     | correctness、skeptic，Python 文件   |
| `typescript-advanced-types`                      | `wshobson/agents`                   | 83.9K            | correctness、skeptic，JS 和 TS 文件 |
| `security-review`                                | `getsentry/skills`                  | 18.9K            | security                            |
| `golang-security`                                | `samber/cc-skills-golang`           | 42.4K            | security，Go 文件                   |
| `git-commit`                                     | `github/awesome-copilot`            | 45.9K            | 仅 `eyeful commit`                  |

安装量是 2026 年 10 月的数据。

`workflow/prompts/tools.yaml` 定义了所有工具，每个角色文件写明这个角色能用哪些工具、用哪个工具提交结果。云端运行时恰好提供这些工具。本地审查没有工具服务：diff 在输入里；审查期间 eyeful 把 skill 写进一个临时目录，输入只列出每个提供的 skill 的名字、描述和路径，agent 用自己的工具读用得上的 skill 和其他文件，eyeful 无法得知本地专家读了哪些 skill，所以本地审查的不自信清单里不列未加载的 skill。agent 最后给出一段 JSON，内容就是提交工具本来要收到的参数（见 [agent](LOCAL.md#agents)）。`git_log`、`git_blame` 和 `try_repro` 在本地没有对应物，所以本地的复现 agent 交出没有运行过的测试，由验证器去运行；测试失败的原因和声称的不一样时，验证器会要求修正一次（见[验证器](VERIFICATION.md#the-verifier)）。

| 工具                                           | 给 agent 提供什么                                  | 角色                         |
| ---------------------------------------------- | -------------------------------------------------- | ---------------------------- |
| `git_diff`                                     | 改动的 diff，可以只看一个文件                      | 除汇总以外的所有角色         |
| `git_log`                                      | 改动之前的提交历史                                 | CI agent、规划器、专家、评判 |
| `git_blame`                                    | 一段代码行最后是谁改的                             | 专家、修复 agent、评判       |
| `list_skills`、`load_skill`、`read_skill_file` | 提供给它的 skill 及其参考文件                      | 专家、复现 agent             |
| `try_repro`                                    | 用确认过的 `test_one` 在一份新副本上跑一次复现测试 | 复现 agent                   |
| `submit_plan`、`submit_report` 等              | 这个角色的结果                                     | 每个角色一个                 |

agent 自带的读文件工具照常可用。eyeful 会拒绝它修改文件或运行命令的请求，所以真正运行的命令只有 eyeful 定好的那些。在云端，提交工具会检查收到的内容，把发现的问题返回给 agent，让它在同一个会话里改正；`submit_report` 做的检查和 Go 对报告的检查相同。本地在收到回复时做同样的检查，被拒绝的报告会退回给专家一次。L1 工具、`find_references` 和 `lookup_standard` 还在计划中。

下文标有 L1 的工具在分诊时就会对每次审查运行，那时还没有任何专家启动。其他工具只在专家调用时运行。

## 让专家记住标准 {#context}

标准通常很长，agent 在长上下文里容易忘掉前面读过的内容。所以 eyeful 不把标准放进提示词，而是让专家加载提供给它的 skill，只在需要时再读 skill 里的参考文件。

1. 一个会话只审查一个改动分组，带着这组文件和为它们提供的 skill；专家文件里没有写 `blind` 的，还带着审查计划。一个分组不会跨越多个[范围](PLANNER.md#large)。用 `execution` 时，能报告 `repro_test` 发现的专家还会被告知改动弄坏了项目的哪些测试（见[被改动弄坏的测试](VERIFICATION.md#regressions)）。
2. 专家要读完自己文件里每一处改动，把这次改动引入的每个问题都报出来，不需要证明：报错一条只多花一次检查，漏掉的问题却没有人能再找到。每条发现的严重度由专家自己定，从 `critical` 到 `low`，其中 `low` 用于不改变任何行为的问题：命名、风格、措辞或小的简化。没有任何规则要求专家略去某一类问题。改动删掉了仍然有人依赖的行为（留在仓库里的调用方或测试，或公开接口的使用者），提交信息又没有声明，也算一条发现。一条发现就是一个问题：它的文件和行号指向最能说明问题的地方，`related` 列出这个问题出现的其他改动位置。同一个错误的改法在三个同类文件里重复出现，是一条发现加两个相关位置，只检查一次。相关位置不在该专家的分组里时，Go 会拒收。每个发现要写明依据的是哪个 skill，或者写 `other`，再给一个简短的类别，适用时加上 CWE 编号。能用测试说明的发现（`repro_test`）和带触发路径的发现还要写出场景：触发它的调用或输入、实际发生了什么、本该发生什么。复现 agent 根据它来写测试（见[验证器](VERIFICATION.md#the-verifier)）。引用了没有提供给这个专家的 skill 的发现会被拒收，这样专家就没法凭空编一条标准。以下发现也会被拒收：文件不在该专家的分组里；缺少行号或标题；证据类型不在专家文件列出的范围内；声称 `cve`、`api_break` 或 `coverage` 证据，但该文件上没有对应的 `dependency_audit`、`openapi_diff` 或 `coverage_diff` 结果；对应的工具在分组里什么都没发现时，这三种证据类型根本不会提供给专家。被拒的报告会连同原因退回给专家重写一次。第二次报告里仍然不合格的发现会被丢弃并列进不自信清单，同一份报告里合格的发现照常保留。
3. eyeful 会记录每个专家用 `load_skill` 加载过哪些 skill。提供了却没有加载的 skill 会列进不自信清单，因为它对应的标准可能没有被用上。本地审查看不到专家读了哪些 skill，所以不会把任何 skill 列为未读。

## design

design 专家看改动放在这里是否合适，和系统其他部分是否协调：各部分之间的配合是否合理，是否应该放进某个库里，新加的抽象是现在就需要还是“以防万一”，某个函数或类型是否比必要的更难理解。

它依据 Google 关于设计和复杂度的问题、耦合与内聚、SOLID 原则、McCabe 圈复杂度，以及项目的架构决策记录（如果有）。skill 有 `codebase-design`，Go 文件再加上 `golang-design-patterns`，Python 文件加上 `python-design-patterns`，JavaScript 和 TypeScript 文件加上 `nodejs-backend-patterns`；项目有 `ARCHITECTURE` 文档时也会读。工具有 `complexity`（L1，用 lizard 分析改动过的函数）、`dependency_graph`（改动后哪些包依赖哪些包）和 `find_references`。

设计方面的发现大多只能靠论证，所以一般写成 `suggestion`、`question` 或 `thought`，除非某项指标超过了项目设定的阈值。每条设计意见都要写明依据哪条原则。

## correctness

correctness 专家看改动是否实现了作者的本意：边界值，空输入和超大输入，错误处理路径，并发问题（竞态、死锁、执行顺序），资源泄漏，以及 `find_references` 找到的每个调用点上的行为。

它依据 Google 关于功能的问题、对应语言的 SEI CERT 编码标准（如果有），并用 CWE 给每个缺陷分类。skill 有 `code-review`，Go 文件再加上 `golang-error-handling` 和 `golang-concurrency`，Python 文件加上 `python-error-handling` 和 `async-python-patterns`，JavaScript 和 TypeScript 文件加上 `typescript-advanced-types`。工具有 `typecheck`（L1：`tsc --noEmit`、`go vet`、`mypy`，计划中）、`static_analysis`（按语言选 `staticcheck` 或 ESLint 的正确性规则，计划中）。

怀疑有功能错误时，它报出能说明问题的那次调用：输入和错误的结果。复现 agent 把它写成测试，由验证器负责运行（见[证据与验证](VERIFICATION.md)）。

## skeptic

skeptic 和 correctness 专家看的是同一块，但从另一边看。没有人告诉它这次改动是为了什么：它的输入里只有分给它的文件，没有审查计划，它像依赖这段代码的人那样去读 diff。对改动删掉或改写的每一行，它找原来依赖旧行为的东西（调用方、子类、测试、文档、已存储的数据）；对新增的每一行，它找作者没有料到的输入、状态或调用顺序。只要有人会因此出问题，哪怕改动看起来是有意为之，它也会报出来。

它的 skill 和 correctness 专家相同，因为两者依据的是同样的标准；它没有自己的工具。两个专家从不同的路走到同一处代码，算作相互印证（见[报告](REPORT.md)）；只有 skeptic 报出来的，就是那份共同的改动说明遮住的东西。

## security

security 专家看改动是否引入了漏洞：注入、访问控制失效、不安全的反序列化、SSRF、密钥泄露，以及使用有已知漏洞的依赖。

它依据 OWASP Top 10、OWASP ASVS（用来指出每个发现违反了哪条要求）、CWE Top 25，以及对应语言的 SEI CERT 编码标准（如果有）。skill 有 `security-review`，它的参考资料依据 OWASP Cheat Sheet Series，覆盖 JavaScript 和 Python；Go 文件再加上 `golang-security`。工具有 `secret_scan`（L1，用 gitleaks 扫 diff）、`dependency_audit`（L1：根据锁文件选 `pnpm audit`、`npm audit`、`govulncheck`、`pip-audit` 或 `cargo audit`）、`cve_lookup`（查询每个新增或升级依赖的 OSV 记录）和 `sast`（对改动的文件运行 Semgrep 的 OWASP 规则）。

已知 CVE 和泄露的密钥本身就是证据。其他发现需要一次测试能做的调用，或者触发路径。每个发现都要带上 CWE 编号。

## tests

tests 专家看改动里的测试值不值得保留：代码出错时测试会不会失败，断言有没有意义，会不会因为无关的原因失败，以及哪些改动过的行没有被任何测试执行到。

它依据 Google 关于测试的问题、ISO/IEC/IEEE 29119 的测试设计技术、van Deursen 等人整理的测试坏味道，以及变异测试。skill 有 `tdd`，Go 文件再加上 `golang-testing`，Python 文件加上 `python-testing-patterns`，JavaScript 和 TypeScript 文件加上 `javascript-testing-patterns`，同时遵循项目自己的测试约定。工具有 `coverage_diff`（L1，把 `coverage` 命令的结果和 diff 逐行对照）、`test_on_base`（在基础提交上运行改动新增的测试，测新行为的测试在那里应该失败）和 `mutate`（只对改动的行运行 Stryker、mutmut 或 go-mutesting）。

如果一个新测试在基础提交上也能通过，或者把它要测的那一行变异之后它仍然通过，那它其实没有测到新东西。覆盖率不足只作为 `suggestion` 提出，这个专家不负责补写测试。

## usability

usability 专家从使用者的角度看改动，使用者包括界面的用户、调用 API 的开发者，以及查看错误和日志的运维人员。

它依据 WCAG 2.2 和 WAI-ARIA 编写实践（无障碍），Nielsen 的十条可用性原则和 ISO 9241-110 交互原则（交互设计），RFC 9457（错误响应），以及语义化版本（API 变更）。skill 是 `accessibility`，覆盖 WCAG 2.2，只在改动涉及页面和组件文件时提供。工具有 `openapi_diff`（L1，比较基础和 head 两版 API 文档中的破坏性变更）、`a11y_check`（对改动涉及的页面运行 axe）和 `e2e_run`（运行 `e2e` 命令并截图）。

API 的破坏性变更和 axe 报出的违规本身就是证据，引用时注明对应的 WCAG 成功准则。基于可用性原则的发现要附上截图，并写明是哪条原则。

## readability

readability 专家看下一位工程师能不能读懂这次改动：命名是否说清了是什么，注释是在解释原因而不是复述代码，写法和周围代码是否一致，行为变了文档有没有跟着改。

它首先依据项目自己的风格指南和贡献指南；没有的话，依据语言官方的约定（Effective Go、PEP 8、Google 风格指南），文档方面依据 Diátaxis。Go 文件的 skill 是 `golang-code-style` 和 `golang-naming`，Python 文件是 `python-code-style`，JavaScript 和 TypeScript 文件是 `modern-javascript-patterns`；其他语言依据项目自己的风格指南。工具有 `lint`（L1，对改动的文件运行项目的 linter）和 `similar_code`（看仓库里同样的事是怎么写的）。

风格指南里明确写了的规则，违反时记为 `issue`；指南里没写的只能算 `nitpick`，不会阻塞。

## 专家文件

每个专家对应 `workflow/prompts/experts/` 下的一个 Markdown 文件，内嵌在二进制文件里。frontmatter 是专家名单里的条目和它用的 skill，正文就是给专家的提示词。`workflow/prompts/roles/expert.md` 补上所有专家共用的部分：怎样使用工具，以及一个发现必须包含什么。`security.md` 的内容：

```markdown
---
name: security
area: security
tier: strong
evidence: [repro_test, cve, trigger_path, argument]
skills:
  - name: security-review
  - name: golang-security
    files: ['**/*.go']
---

You are the security expert on a code review. You ask one question: does this change introduce a
vulnerability? Follow untrusted input from where it enters to where it is used. ...
```

`area` 是这个专家的评论带上的 decoration。

增加一个专家就是增加一个文件。它能不能留下，由[效果评测](EVALUATION.md#ablations)决定：去掉它之后测量结果没有变化，就删掉。

## 专家能看到什么

专家拿到的是：diff、计划分给它的文件、审查计划、这些文件的 L1 工具结果、它的 skill，以及它调用的工具的输出。它看不到其他专家的输出，所以两个专家报告了同一个问题，说明它们是各自独立发现的；只有评测的 `--dependent` [消融](EVALUATION.md#ablations)会让专家依次运行，并把前面的发现给后面的专家看。每个专家在单独的进程里运行。

## 专家出错时

专家超时或出错，就会被去掉。其他专家继续工作，反馈里会把它列为缺席，读者可以知道哪部分没有被审查，审查结果记为 `partial`。第二次提交的报告里仍被 Go 拒收的发现，只丢掉这些发现本身。
