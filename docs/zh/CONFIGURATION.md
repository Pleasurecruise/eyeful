# 配置项参考

eyeful 自己找出被审查项目的构建和测试方法。本地设置文件保存用户为 `eyeful review` 做的选择；云端 action 通过环境变量和命令行参数配置。

## 项目命令 {#project}

eyeful 用规则、不用模型，根据快照根目录下的文件选出命令，按表格顺序取第一个匹配的行。

| 文件                                                       | `setup`                                                 | `test`、`test_one`                                                                  | `lint`                                            |
| ---------------------------------------------------------- | ------------------------------------------------------- | ----------------------------------------------------------------------------------- | ------------------------------------------------- |
| `go.mod`                                                   | `go mod download`                                       | `go test ./...`，单个测试加 `-run {test}`                                           | 有 `.golangci` 文件时用 `golangci-lint run ./...` |
| `Cargo.toml`                                               | `cargo fetch`                                           | `cargo test`，单个测试加 `{test}`                                                   |                                                   |
| 提到 pytest 的 `pyproject.toml`、`setup.py` 或 `setup.cfg` | 按锁文件用 `uv sync`、`poetry install` 或 `pip install` | `pytest -q`，单个测试加 `{test}`；用 uv 或 poetry 安装时经由 `uv run`、`poetry run` | 配置了 ruff 时用 `ruff check .`                   |
| `package.json`                                             | 用锁文件对应的包管理器安装                              | 依赖里有 vitest、jest 或 mocha 时用它们，单个测试加 `{test}`；否则只用 `test` 脚本  | `lint` 脚本                                       |

| 命令       | 用在哪里                                                     |
| ---------- | ------------------------------------------------------------ |
| `setup`    | 准备阶段：安装依赖                                           |
| `lint`     | 本地 CI 校验                                                 |
| `test`     | 验证器运行整套测试：审查前找出改动弄坏的测试，每次修复后再跑 |
| `test_one` | 验证器运行单个复现测试，`{test}` 会替换成加了引号的测试编号  |

一行都匹配不上的项目，或者没有提到 pytest 的 Python 项目，没有命令：什么都不运行，所有发现都由裁判阅读，审查开始前会说明这一点。

`eyeful review` 的 `--setup`、`--lint`、`--test` 和 `--test-one` 会替换 eyeful 找到的命令，`--no-detect` 则只运行给出的命令。评估用它们在容器里运行 SWE-bench 的测试。

审查时执行的项目代码只有这些命令，而且要在审查开始前经过用户确认（见[开始之前](REVIEW.md#before-it-starts)）。

仓库的 `.gitattributes` 标为 `linguist-generated` 或 `linguist-vendored` 的文件不交给任何专家（见[分拣](PLANNER.md#triage)）。

## 本地设置 {#local}

`eyeful connect` 会写入用户配置目录下的 `eyeful/settings.json`（macOS 上是 `~/Library/Application Support`，Linux 上是 `$XDG_CONFIG_HOME` 或 `~/.config`）：

```json
{
	"agent": "claude",
	"repository": "/Users/me/src/app"
}
```

没有连接 agent 时 `eyeful review` 会报错。`repository` 是桌面端上一次显示的仓库，桌面端不在仓库里启动时会打开它。设置文件旁边的 `eyeful/confirmed` 为每次确认保存一个哈希，由仓库路径和在那里确认过的命令算出，所以仓库没法自带一份确认记录。每个项目在 `.eyeful/runs/` 下保存历次审查的运行记录。这个目录里有一个内容为 `*` 的 `.gitignore`，所以 git 看不到它，它也不会被纳入审查。

## Action 配置 {#action}

`eyeful-action` 的每个命令行参数都有对应的环境变量，`eyeful-action --help` 会同时列出两者。`action.yml` 从它的输入和 job 环境里设置它们。缺少必需的值时直接报错，不会猜一个默认值。

### 密钥 {#secrets}

| 密钥                      | 用途                                                       |
| ------------------------- | ---------------------------------------------------------- |
| `EYEFUL_MODEL_KEY`        | `EYEFUL_MODEL_URL` 上 provider 的 API key。                |
| `EYEFUL_TYPESAFE_API_KEY` | Jev 用的 TypeSafe API key。可选；没有时不合并发现。        |
| `EYEFUL_GITHUB_TOKEN`     | job 的 `GITHUB_TOKEN`，用来克隆、读取 Actions 和发布审查。 |

在 GitHub 上，前两个是传给 action 输入的仓库 secrets（见[部署](cloud/DEPLOYMENT.md#turn-on-reviews)）。在本地它们放在根目录的 `.env` 里，git 忽略它，mise 以 `redact` 方式加载；`.env.example` 列出了它们。`eyeful review` 用 `EYEFUL_TYPESAFE_API_KEY` 调用 [Jev](ARCHITECTURE.md#jev)；没有它时不合并发现。

### 变量 {#variables}

不是密钥的配置都在这里。`DOCS_URL` 写在 `mise.toml` 的 `[env]` 中。

| 变量                 | 用途                                                                        |
| -------------------- | --------------------------------------------------------------------------- |
| `EYEFUL_PULL`        | 要审查的 pull request。                                                     |
| `GITHUB_REPOSITORY`  | 仓库，格式 `owner/name`。由 Actions 设置。                                  |
| `GITHUB_RUN_ID`      | workflow 运行；它标记审查发布的内容，审查也不会等它自己。                   |
| `GITHUB_API_URL`     | GitHub API 根地址。默认 `https://api.github.com`。                          |
| `EYEFUL_DIR`         | 检出和审查文件所在目录，以只读方式共享给沙箱。默认 `/work`。                |
| `EYEFUL_SANDBOX_URL` | 这次审查的沙箱。默认 `http://eyeful-sandbox:8080`。                         |
| `EYEFUL_MODEL_URL`   | 提供 pi 所用模型的 provider 的 OpenAI 兼容地址；是仓库变量。                |
| `EYEFUL_AGENT`       | 审查连接的 agent CLI：`pi`、`claude` 或 `codex`。默认 `pi`。                |
| `EYEFUL_LOG_LEVEL`   | `debug`、`info`、`warn` 或 `error`。默认 `info`。                           |
| `DOCS_URL`           | 文档站的公开地址，用于 sitemap、robots 和 `llms.txt`。写在 `mise.toml` 里。 |

云端审查用的模型写在 `cmd/eyeful-action/pi/` 里 pi 自己的设置中，不由变量决定（见[agent 与模型](cloud/CLOUD.md#agents-and-models)）。
