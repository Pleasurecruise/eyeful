# 配置项参考

配置分三类。被审查仓库里的 `.eyeful/config.yml` 告诉审查怎样构建和测试这个项目；本地设置文件保存用户为 `eyeful review` 做的选择；服务端则通过环境变量和命令行参数配置。

## 项目配置 {#project}

`.eyeful/config.yml` 放在被审查的仓库里，eyeful 也为审查自己准备了一份 [`.eyeful/config.yml`](../.eyeful/config.yml)。`eyeful review` 会读取它，出现未知的键时报错。云端还不读取它。

```yaml
setup: pip install -e ".[test]"
lint: ruff check .
test: pytest -x -q --junitxml=report.xml
test_one: pytest -x -q {test}
coverage: pytest --cov --cov-report=json
e2e: npx playwright test
risk: ['auth/**', 'payments/**', 'migrations/**']
skip: ['docs/api/**']
```

| 键         | 用在哪里                                                     |
| ---------- | ------------------------------------------------------------ |
| `setup`    | 准备阶段：安装依赖。在云端，这是唯一能访问包仓库的步骤       |
| `lint`     | 本地 CI 校验，以及 readability 专家的 `lint` 工具            |
| `test`     | 验证器运行整套测试：审查前找出改动弄坏的测试，每次修复后再跑 |
| `test_one` | 验证器运行单个复现测试，`{test}` 会替换成加了引号的测试编号  |
| `coverage` | tests 专家的 `coverage_diff` 工具                            |
| `e2e`      | usability 专家的 `e2e_run` 工具                              |
| `risk`     | 审查计划：涉及这些路径的分组是核心组                         |
| `skip`     | 分诊：这些路径不交给专家，在反馈中列为未审查                 |

审查时执行的项目代码只有这些命令，而且要在审查开始前经过用户确认（见[开始之前](REVIEW.md#before-it-starts)）。没有 `test` 和 `test_one` 时，发现都保持未验证。

## 本地设置 {#local}

`eyeful connect` 会写入用户配置目录下的 `eyeful/settings.json`（macOS 上是 `~/Library/Application Support`，Linux 上是 `$XDG_CONFIG_HOME` 或 `~/.config`）：

```json
{
	"agent": "claude",
	"repository": "/Users/me/src/app"
}
```

没有连接 agent 时 `eyeful review` 会报错。`repository` 是桌面端上一次显示的仓库，桌面端不在仓库里启动时会打开它。设置文件旁边的 `eyeful/confirmed` 为每次确认保存一个哈希，由仓库路径和在那里确认过的命令算出，所以仓库没法自带一份确认记录。每个项目在 `.eyeful/runs/` 下保存历次审查的运行记录。这个目录里有一个内容为 `*` 的 `.gitignore`，所以 git 看不到它，它也不会被纳入审查。

## 服务端配置 {#server}

每个命令行参数都有对应的环境变量，`eyeful-server serve --help` 会同时列出两者。密钥和其他变量分开存放，都放在仓库根目录，任何应用或包都没有自己的配置文件。缺少必需的值时直接报错，不会猜一个默认值。

### 密钥：`.env` {#secrets}

`.env` 不进 git。mise 加载它时使用 `redact`，所以值不会出现在任务输出里。`.env.example` 列出的正好是这些项。

| 密钥                       | 用途                                                                                 |
| -------------------------- | ------------------------------------------------------------------------------------ |
| `EYEFUL_DATABASE_URL`      | PostgreSQL 地址。必需。                                                              |
| `EYEFUL_TEST_DATABASE_URL` | 允许 `go test` 清空的数据库。不设置时跳过这些测试。                                  |
| `EYEFUL_TYPESAFE_API_KEY`  | TypeSafe 的 API 密钥，供 [Jev](ARCHITECTURE.md#jev) 使用。可选：没有它时不合并发现。 |

TODO(source)：接入 webhook 时，GitHub App 的 ID、私钥和 webhook secret 加进这张表。

### 变量：`mise.toml` 和命令行参数 {#variables}

不是密钥的配置都在这里。`DOCS_URL` 写在 `mise.toml` 的 `[env]` 中；在服务器上，其余的值来自部署平台的环境变量。

| 变量               | 用途                                                                            |
| ------------------ | ------------------------------------------------------------------------------- |
| `EYEFUL_ADDR`      | 监听地址。默认 `:8080`。                                                        |
| `EYEFUL_LOG_LEVEL` | `debug`、`info`、`warn` 或 `error`。默认 `info`。                               |
| `DOCS_URL`         | 文档站点的公开地址，用于 sitemap、robots 和 `llms.txt`。在 `mise.toml` 中设置。 |

只有命令行参数、没有对应环境变量的配置：`--workers`（2）、`--lease-ttl`（`30s`）、`--poll`（`1s`）、`--max-attempts`（3）、`--shutdown-timeout`（`30s`）和 `--no-migrate`。工作进程数、租约时长和轮询间隔必须为正数，否则服务端拒绝启动。TODO(config)：等配置出现嵌套结构（模型档次、沙箱）后，改为 `conf/eyeful.example.toml`。
