# 代码规范与评审

本页是 eyeful 自身代码的规则。Go 部分的规则出自该节引用的资料，最后的评审步骤用来在宣布改动完成之前检查它。

## 代码结构

- 包以它负责的能力命名；不要 `utils`、`helpers`、`common`、`manager`、`service`。依赖方向见[整体架构](ARCHITECTURE.md#dependency-direction)。
- 每个包把自己的类型、枚举、sentinel error 和消费方接口放在 `types.go`。
- 构造函数是参数带类型的普通函数；只有组合根（`internal/app`、`local/app`）引入 fx。没有包级状态。
- 接口属于它的消费方，只列出消费方会调用的方法。
- HTTP handler 按 CRUD 组织，每个资源一个文件，顺序为 create、list、get、update、delete。

## 命名约定

| 对象                          | 规则                                  | 示例                            |
| ----------------------------- | ------------------------------------- | ------------------------------- |
| Go 包、目录                   | 小写、单个单词                        | `review`、`apperror`            |
| Go 文件                       | 小写，必要时才用 `snake_case`         | `memory_test.go`                |
| Go 导出 / 未导出标识符        | `PascalCase` / `camelCase`            | `NewWorkerPool`、`consoleRoute` |
| Go 缩写词                     | 保持同一大小写                        | `ID`、`URL`、`reviewID`         |
| Go 接收者                     | 一到两个字母，同一类型保持一致        | `s *MemoryStore`                |
| Go 测试                       | `TestXxx`，一到三个词；场景放进子测试 | `TestFencing`                   |
| JSON 字段、查询参数           | `snake_case`                          | `created_at`                    |
| URL 路径                      | 小写复数名词，`kebab-case`            | `/api-keys/{id}`                |
| Operation ID、SDK 函数        | `camelCase` 动词 + 名词               | `updateReview`                  |
| Schema、TS 类型、组件         | `PascalCase`                          | `Review`、`ReviewTable.svelte`  |
| TS 变量、函数                 | `camelCase`                           | `tokenKey`                      |
| 错误码、SQL                   | `snake_case`                          | `review_not_found`              |
| 环境变量                      | `UPPER_SNAKE_CASE`，前缀 `EYEFUL_`    | `EYEFUL_DATABASE_URL`           |
| CLI 参数、mise 任务、npm 包名 | `kebab-case`（任务变体用 `:`）        | `--critical-rate-limit`         |
| 文档                          | `UPPERCASE.md`                        | `SCHEDULING.md`                 |

名字要短且没有歧义，让包名承担上下文（`review.Store`，而不是 `review.ReviewStore`）。

## 辅助函数

逻辑写在使用它的地方。单独抽成函数需要理由：三处以上调用、外部边界、框架回调、具名的领域操作或不变量，或者复杂到需要单独测试。只有一个调用方的包装、给表达式换个名字、隐藏控制流的函数，一律内联。增加缓存、重试层、开关或依赖之前，先说明没有它会出什么问题。

## 依赖与版本

工具链在 `mise.toml` 中跟随 `latest`，这是唯一声明工具版本的地方。`package.json` 中不写 `devEngines` 或 `packageManager`，生成器自动加上的也要删掉。库在添加时取最新版本并精确固定（`go get …@latest`；`pnpm-workspace.yaml` 中设置 `saveExact`，不用 `^` 或 `~`）。例外记录在[本地开发](DEVELOPMENT.md#toolchain-notes)中。升级单独提交。

## Go 代码

- gofmt、goimports、golangci-lint 全部干净；例外写在 `.golangci.yml`，不使用 `//nolint`。
- 除了 `TODO(area): what`（一行，完成时删除）、swag 注释和 `//go:` 指令之外不写注释。`grep -rn "TODO(" cmd internal workflow local apps` 就是待办清单。
- 错误用 `%w` 包装，用 `errors.Is`/`As` 判断；不要转成一个看似合理的默认值。返回给客户端的错误是 `apperror` 错误码；其余一律记录日志并返回 500。
- 涉及 I/O 的函数第一个参数是 `context.Context`，不存进结构体，并约束每一次外部调用。不在持有锁时做 I/O。用 `slog` 记录日志，不记录密钥。
- CLI 命令是带 `help`、`default`、`env` 标签和 `Run() error` 方法的 kong 结构体。

下表列出要避免的写法，出自 [Effective Go](https://go.dev/doc/effective_go)、[Code Review Comments](https://go.dev/wiki/CodeReviewComments) 和 [Google 风格指南](https://google.github.io/styleguide/go/)，并对照 TypeScript 里的类似习惯：

| TypeScript 习惯  | Go 中要避免                                         | 替代做法                                                      |
| ---------------- | --------------------------------------------------- | ------------------------------------------------------------- |
| `any`、`unknown` | `any`、`interface{}`、`[T any]`、`map[string]any`   | 具体类型；联合约束；`json.RawMessage`；`TODO(area): type TBD` |
| `as`             | 不带 `, ok` 的 `x.(T)`；未校验的枚举转换；`reflect` | `, ok`、type switch、`ParseX(string) (X, error)`              |
| `try/catch`      | HTTP 中间件之外的 `recover()`；`panic`              | 返回 `error`；只在一处处理                                    |
| 吞掉错误         | `_ = f()`、`v, _ := f()`                            | 处理，或者包装后返回                                          |
| `undefined`、`!` | 未检查的 nil；仅为表示"可选"而用指针                | 零值；`(T, bool)`；只有在传输层确实需要表示缺失时才用指针     |

## TypeScript 与 Svelte

- 从官方脚手架开始，只修改 eyeful 需要的部分。
- 一切都由 Vite+ 运行：`vp dev`、`vp build`、`vp fmt`（Oxfmt）、`vp lint`（类型感知的 Oxlint）、`vp check`。配置都在根目录的 `vite.config.ts`。
- 类型检查使用 TypeScript 7。传输类型来自 `@eyeful/sdk`，不手写。
- 不使用 `any`、非空断言，也不用类型转换绕过校验。

## 安全要求

鉴权、会话和凭据遵循当前的 OWASP ASVS。比较密钥时使用常量时间比较，或者只存储哈希。密钥不进入沙箱、日志、错误响应或控制台的构建产物。配置和密钥只放在仓库根目录：密钥在 `.env`，共享设置在 `mise.toml`；各个 app 和 package 都不单独放 env 文件，代码也不兜底一个内置值。

## 测试要求

Go 测试与代码放在一起（`*_test.go`，见 `go help test`），默认黑盒（`package x_test`），除非要用到未导出的接缝；采用表驱动，断言就是普通的 `if`。handler 通过 `httptest` 针对真实的服务端测试。网络、凭据和数据库都是可选启用的。修 bug 从一个失败的测试开始；并发测试要覆盖关键的执行顺序。

## 如何评审改动 {#review}

在认为一个改动完成之前，试着扮演以下角色去破坏它：

1. 恶意用户：构造特殊输入，或者跳过某些步骤；
2. 制造极端数据的人：空值、超大、重复、乱序、Unicode；
3. 网络不稳定的用户：超时、重试、重复提交；
4. 没有权限的用户：凭据缺失、错误或过期；
5. 之后接手这段代码的新工程师。

评审由没参与实现的人来做，他先看到问题本身，再看作者的诊断。双方各自写下结论，有分歧时，写一个能区分两种说法的测试来判断。还要确认：改动解决的是有人实际遇到的问题，事实和推断分开写了，没有可以删掉的部分，并且没有违反 `.agents/POLARIS.md` 中的硬性指标。每次交接最后都附上不自信清单：运行了什么、哪些没测、哪些还是假设，以及怎样才能确认。
