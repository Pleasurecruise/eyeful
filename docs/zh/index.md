---
layout: home

hero:
  name: eyeful
  text: 以可执行证据为基础的多智能体代码审查
  tagline: '「只要眼睛足够多，bug 都是浅显的。」大多数项目没有那么多眼睛，eyeful 帮它们补上。'
  image:
    src: /eyeful.png
    alt: eyeful 水獭标志
  actions:
    - theme: brand
      text: 快速开始
      link: /zh/LOCAL
    - theme: alt
      text: 审查如何进行
      link: /zh/REVIEW
    - theme: alt
      text: GitHub
      link: https://github.com/Pleasurecruise/eyeful

features:
  - title: 用运行结果说话
    details: 专家认为有 bug，就要写测试证明，由 eyeful 来运行。最有力的发现在改动上失败、加上修复后通过。
    link: /zh/VERIFICATION
  - title: 先读懂，再审查
    details: pulls.review core 先弄清这次改动干了什么，按意图分组并标出核心部分。Go 给每组挑选专家，每个专家只运行一次，审查分给它的组，对照所在领域的公开标准。
    link: /zh/EXPERTS
  - title: 用你自己的编码 agent
    details: 用你已经登录的 Claude Code、Codex 或 pi 审查，通过 eyeful 命令或桌面端运行。不需要账号和服务器。
    link: /zh/LOCAL
  - title: 不动你的工作区
    details: eyeful 审查固定下来的快照，只运行你确认过的项目命令，而且只在一份用完即删的检出里运行。
    link: /zh/LOCAL#local-risks
---

本地审查已经可用，针对 pull request 的云端审查在计划中。建议先读 [eyeful 是什么](/zh/overview)，或看[版本路线图](/zh/ROADMAP)。
