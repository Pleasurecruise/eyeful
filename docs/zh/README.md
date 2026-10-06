# eyeful 是什么

eyeful 用一组 agent 审查代码改动。规划器挑出改动需要的专家，专家对照公开标准并行审查；专家认为有 bug 时要写测试证明，由 eyeful 运行。结果以评审意见的形式交给人。

```mermaid
flowchart LR
    change(["改动"]) --> plan["规划<br/>CI、分诊、计划"]
    plan --> experts["专家<br/>并行审查"]
    experts --> verify["验证<br/>运行测试"]
    verify --> feedback(["评审意见"])
```

## 当前进度

本地审查已经可用：`eyeful review`、`provider`、`connect`、`commit` 和桌面端用你自己的编码 agent 跑完整个流程（见[本地运行](LOCAL.md)）。云端审查（pi 中的 agent、沙箱）、L1 工具和各专家自己的工具还在计划中，相关页面都会注明，顺序见[版本路线图](ROADMAP.md)。

## 文档导航

审查怎么进行：

| 文档                          | 内容                                              |
| ----------------------------- | ------------------------------------------------- |
| [一次审查的全过程](REVIEW.md) | 确认、各阶段、结果、评审标准、规则                |
| [规划与分诊](PLANNER.md)      | CI 校验、分诊、大改动、审查计划，以及 Go 怎样检查 |
| [档位](LEVELS.md)             | 四个层级，以及每个档位运行什么                    |
| [专家](EXPERTS.md)            | 每个专家的检查清单、工具和证据                    |
| [证据与验证](VERIFICATION.md) | 判定依据、证据类型、验证器                        |
| [审查反馈](REPORT.md)         | 评论格式、行内评论、排序报告、多轮审查            |
| [本地运行](LOCAL.md)          | 桌面端和命令行、快照、agent 命令行、风险          |
| [云端运行](CLOUD.md)          | pull request、在 pi 中运行 agent、沙箱            |
| [常见问题](FAQ.md)            | 审查什么、改动过大、审查期间文件被改动            |
| [参考文献](REFERENCES.md)     | 评审、安全和无障碍方面的标准，借鉴的项目          |
| [八条原则](PRINCIPLES.md)     | 作者总结的八条工作原则，仅供参考                  |

毕业设计相关：[效果评测](EVALUATION.md)，包括研究问题和实验设计。

代码怎么组织：

| 文档                            | 内容                             |
| ------------------------------- | -------------------------------- |
| [整体架构](ARCHITECTURE.md)     | 组件、目录、依赖方向、生命周期   |
| [任务调度与租约](SCHEDULING.md) | 准入、租约、重试、模型路由、限流 |
| [登录与凭据](AUTH.md)           | 有哪些凭据，怎么保护             |
| [数据库与迁移](PERSISTENCE.md)  | PostgreSQL 表结构、迁移、查询    |
| [API 约定](API.md)              | HTTP 约定、错误、路由、MCP       |
| [本地开发](DEVELOPMENT.md)      | 环境准备、常用命令、代码生成     |
| [代码规范与评审](STYLEGUIDE.md) | 编码规则，以及改动怎么评审       |

部署相关：[部署上线](DEPLOYMENT.md)和[配置项参考](CONFIGURATION.md)。后续计划见[版本路线图](ROADMAP.md)。

## 关于这些文档

每篇文档只讲一个主题，描述代码现在的行为，未实现的内容会注明。英文是源文本，`zh/` 是中文译本，由 `mise run lint` 保证两边一致。站点用 VitePress 构建（`mise run dev:docs`），从 `main` 发布到 GitHub Pages。
