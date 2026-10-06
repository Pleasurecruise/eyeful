# 云端运行

云端审查在 eyeful 服务端上进行，审查对象是一个 GitHub pull request。agent 使用服务端上配置的模型密钥，项目命令在为每次审查单独创建的沙箱里运行。服务端、登录和审查队列已经可用，审查本身尚未实现。

## 可以审查什么 {#what-can-be-reviewed}

只能是用户授权给 eyeful GitHub App 的仓库中的 pull request，不审查分支、提交或未提交的改动。云端不会从用户电脑上读取任何东西，这些都在本地审查（见[本地运行](LOCAL.md)）。

## 云端审查的步骤

1. 用户在控制台确认审查，见[开始之前](REVIEW.md#before-it-starts)。webhook 和定时触发以后再做（见[版本路线图](ROADMAP.md)）。
2. eyeful 读取 head 提交对应的 GitHub Actions 运行记录，还有运行没结束就等待。如果有失败的运行，CI agent 读取失败任务的日志，审查以失败诊断结束（见[规划与分诊](PLANNER.md#ci-check)）。
3. 工作进程克隆仓库，把 pull request 的基础和 head 解析成具体提交，在运行任何东西之前先记下快照（两个提交，以及 head 相对两者合并基点的 diff）。新的推送是新的快照，也是一次新的审查。大型 pull request 和其他改动一样拆成[范围](PLANNER.md#large)，报告里的范围列表就是给它的 stacked PR 建议。
4. 分诊在沙箱里运行规则和 L1 工具，standard 及以上档位再由规划器制定计划。
5. 每个选中的专家在工作进程上作为一个 pi 进程运行。
6. 验证器在沙箱里的新副本上运行复现测试，汇总阶段写出反馈。
7. eyeful 的 GitHub App 把行内评论作为一个 `COMMENT` review 发布，排序报告作为一条评论发布，并按配置附上可选的 Check Run。SARIF 结果也可以上传到 code scanning（见[审查反馈](REPORT.md)）。

## 在 pi 中运行 agent {#agents-in-pi}

每个 agent 是一个 pi 进程，使用全新的 `HOME`，不保存会话，只有对检出代码的只读工具。被审查仓库里提交的 agent 指令文件一律忽略。模型密钥留在沙箱外的 pi 进程里，接触不到被审查的代码。启动一个专家的命令大致如下：

```sh
pi --print --mode json --no-session \
  --model anthropic/claude-sonnet-5-5 \
  --tools read,grep,find_references,secret_scan,dependency_audit,cve_lookup,sast,submit_report \
  --extension eyeful-pi-extension.js \
  --append-system-prompt workflow/prompts/experts/security.md \
  "Review the files the plan assigned to you"
```

`--mode json` 输出事件流供 Go 解析，`--tools` 把 agent 能用的工具限制在专家文件允许的范围内。扩展提供专家工具以及 `submit_plan` 和 `submit_report`。每次工具调用都在沙箱里运行 eyeful 定好的命令，返回格式固定的结果。每个角色用哪个模型，由[模型路由](SCHEDULING.md#agent-runs-over-providers)决定。

## 沙箱 {#the-sandbox}

每次审查一个沙箱。同样的内容再审一次就是一次新的审查，用新的沙箱。沙箱里没有任何凭据，GitHub token、API key、模型密钥都不会进去。它只能在 `setup` 期间访问包仓库，其他网络都不通。每次复现都在新副本上进行，一条发现的测试不会影响另一条。沙箱按所属审查打标签，服务端启动时会清理遗留的沙箱。这些限制是 `.agents/POLARIS.md` 中的硬性指标。

## 账号与费用

云端审查归创建它的用户所有（见[登录与凭据](AUTH.md)）。模型费用使用服务端配置的密钥支付，每次审查都会记录 token 数、费用和耗时。
