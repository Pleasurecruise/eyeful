# 本地开发

## 环境准备 {#setup}

```sh
mise trust && mise install   # Go, Node.js, pnpm, golangci-lint
mise run setup               # go mod download + pnpm install
cp .env.example .env         # secrets only; mise loads and redacts it
createdb eyeful && createdb eyeful_test
```

PostgreSQL 18 可以用 Homebrew 安装（`brew install postgresql@18`，然后 `brew services start postgresql@18`），也可以用 Docker（`mise run db:up`，数据库地址改为 `eyeful:eyeful@localhost`）。桌面端还需要系统的 webview：macOS 自带；Windows 需要 WebView2；Linux 需要 GTK 4 和 WebKitGTK 6.0（Ubuntu 24.04 上是 `libgtk-4-dev` 和 `libwebkitgtk-6.0-dev`）。

## 常用命令 {#commands}

| 命令                                       | 作用                                                                                      |
| ------------------------------------------ | ----------------------------------------------------------------------------------------- |
| `mise run dev:web`（别名 `dev`）           | 在 `:8080` 启动服务端，在 `:5173` 启动支持热更新的控制台；打开 `:5173`                    |
| `mise run dev:server`                      | 只在 `:8080` 启动服务端（会先迁移），控制台用上一次 `build` 的结果                        |
| `mise run build`                           | 先构建控制台，再构建内嵌控制台的 `bin/eyeful-server`                                      |
| `mise run install`                         | 把本地命令行 `eyeful` 安装到 Go 的 bin 目录，mise 会把它加入 `PATH`；升级 Go 后要重新安装 |
| `mise run skills`                          | 按 `skills-lock.json` 重新安装 `workflow/prompts/skills` 里的审查 skill                   |
| `mise run dev:desktop` / `package:desktop` | 分别执行 `wails3 dev`（前端经 Vite 在 `:9245` 热更新）、`package`；不需要服务端           |
| `mise run dev:docs` / `build:docs`         | 文档站点（VitePress，内容来自 `docs/`）                                                   |
| `mise run test`                            | 带 `-race` 运行 Go 测试；Postgres 相关测试需要设置测试数据库地址                          |
| `mise run format`                          | gofmt 和 `vp fmt`                                                                         |
| `mise run lint`                            | gofmt 检查、golangci-lint、`vp check`，以及每个包的 `typecheck`                           |
| `mise run generate`                        | 依次生成 sqlc、spec、SDK，以及桌面端绑定                                                  |
| `mise run check`                           | lint + test + generate，生成的文件有变化时失败                                            |

开发时要登录，需要注册一个 GitHub App，设置如下：

- 回调地址 `http://localhost:5173/oauth/github/callback`（`mise.toml` 把 `EYEFUL_PUBLIC_URL` 设为控制台的开发地址，Vite 会代理 `/oauth`）；
- 打开 Expire user authorization tokens；
- 仓库权限 Contents 和 Metadata，只读；
- 账号权限 Email addresses，只读；
- 不配置 webhook。

计划增加的权限：Actions 和 Checks 只读，用于 [CI 校验](PLANNER.md#ci-check)；Pull requests 读写，用于发布评审意见。

生成一个 client secret，和 client ID、`EYEFUL_TOKEN_KEY=$(openssl rand -base64 32)` 一起写进 `.env`。把应用安装到希望控制台显示的仓库上，然后登录。Gitee OAuth 应用是可选的（回调地址 `…/oauth/gitee/callback`）。表结构改动后要重建数据库（`dropdb eyeful && createdb eyeful`）。

## 配置

配置只放在仓库根目录：凭据放 `.env`，本地共用的设置放 `mise.toml`。全部变量和参数见[配置项参考](CONFIGURATION.md#server)。

## 代码生成

sqlc（`generate:db`）生成 `internal/db/sqlc`。swag v2（`generate:sdk` 的第一步）生成 `spec/`，由二进制文件内嵌。`@hey-api/openapi-ts`（`generate:sdk`）生成 `packages/sdk/src`。`packages/pulls`（`generate:pulls`）把 `@pulls.review/core` 打包成 `workflow/pulls/core.js`。`wails3 generate bindings`（`generate:bindings`）把桌面端的服务以类的形式生成到 `apps/desktop/frontend/bindings`，这是 Wails 的默认位置，Go 的 `nil` 切片到前端是 `[]`；这个目录不进 git，`lint`、`wails3 build` 和 `wails3 dev` 都会重新生成。这些生成结果都不要手动修改。
桌面图标的源文件是 `apps/desktop/build/appicon.png`；`generate:icons` 会一并生成 dev 和打包构建使用的 macOS `icons.icns`、`Assets.car`，以及 Windows `icon.ico`。

## 多语言

两个前端都使用 [Paraglide JS](https://inlang.com/m/gerre34r/library-inlang-paraglideJs)。每个应用有自己的 `project.inlang/` 和 `messages/{en,zh}.json`，编译到 `src/lib/paraglide`（不进 git）：`dev` 和 `build` 时由 Vite 插件编译，`typecheck` 前由 `prepare` 编译。界面语言依次取 `localStorage`、系统语言，最后是英文，可以在页头的语言菜单里切换。`packages/ui` 本身不含文案，页面把翻译好的文字传进去。往 `en.json` 加文案时，要在同一次改动中加到 `zh.json`。

## 工具链说明 {#toolchain-notes}

- Vite+ 1.0.0 通过 `vp migrate` 接入。pnpm catalog 把 `vite` 指向 `@voidzero-dev/vite-plus-core`。
- 类型检查用 TypeScript 7.0.2（`@typescript/native`、`svelte-check --tsgo`）。`typescript` 保持在 6.0.3，因为 `svelte-kit sync` 和 SDK 生成器要用它的 JS API；它不参与 `pnpm update`。
- VitePress 用的是 2.0.0-alpha.20。1.6.4 在 Vite+ 下会出错，因为它调用了已经移除的 `transformWithEsbuild`，2.x 则适配当前的 Vite。
- swag v2.0.0-rc6 和 Wails v3.0.0-beta.27 还没有正式版。
- 项目用到的工具都是 Go 工具，mise 只安装工具链。sqlc 和 swag 写在根 `go.mod` 的 `tool` 中，用 `go tool` 运行。`wails3` 命令行写在 `apps/desktop/go.mod` 的 `tool` 中，版本始终和 Wails 库一致。`tools:wails` 把它构建到 `.tools/bin`（不进 git），`mise.toml` 只在仓库目录内把这个目录加进 `PATH`，因为 Wails 的 Taskfile 会调用 `wails3`。桌面端的任务会先执行它。

## 重新生成前端

`apps/web`、`apps/desktop/frontend` 和 `packages/ui` 都是生成器创建的：

```sh
pnpm exec vp create svelte --no-interactive --no-agent --no-editor --no-hooks \
  --package-manager pnpm -- web --template minimal --types ts --add sveltekit-adapter="adapter:static"
pnpm exec vp create svelte --no-interactive --no-agent --no-editor --no-hooks \
  --package-manager pnpm -- frontend --template minimal --types ts \
  --add sveltekit-adapter="adapter:static"                 # in apps/desktop
pnpm exec vp create svelte --no-interactive --no-agent --no-editor --no-hooks \
  --package-manager pnpm -- ui --template library --types ts --add tailwindcss="plugins:none"
pnpm dlx shadcn-svelte@1.7.0 init --base-color neutral --css src/lib/app.css \
  --lib-alias '#lib' --components-alias '#lib/components' --ui-alias '#lib/components/ui' \
  --utils-alias '#lib/utils' --hooks-alias '#lib/hooks'   # in packages/ui
pnpm dlx shadcn-svelte@1.7.0 add <component>             # in packages/ui
```

在生成结果之上只做了下面这些修改：

- 三者都有：使用精确版本、`@typescript/native`、带 `--tsgo` 的 `typecheck`；`vite.config.ts` 从 `vite-plus` 导入；删除 `.npmrc`、`.vscode` 和模板自带的 README。
- 两个应用：使用 `@eyeful/ui`；`tailwindcss()`；`router: { type: 'hash' }`；adapter 输出到 `dist/`，`build` 之后执行 `touch dist/.gitkeep`（`go:embed` 要求目录存在）；按 `sv add paraglide` 的方式接入 Paraglide，但去掉它的服务端 hook 和 URL 策略，因为两个应用都是静态的。
- `apps/web`：包名 `@eyeful/web`，使用 `@eyeful/sdk`；为 API 路径配置开发代理；`embed.go`；各页面路由。hash 路由让控制台路径和同源的 API 路由互不冲突。
- `apps/desktop/frontend`：包名 `@eyeful/desktop`，使用 `@wailsio/runtime`；`#bindings/*` 导入；供 `wails3 dev` 使用的 `build:dev`；审查页面。这里必须用 hash 路由，因为 Wails 从 `wails://` 提供页面，它的 origin 是 `null`。
- `apps/desktop/build`：名称、标识符和版本来自 `config.yml` 的 `info`，由 `wails3 task common:update:build-assets` 写入 `Info.plist`、`Info.dev.plist` 以及 Windows 和 Linux 的清单，这些文件不手动修改。绑定生成时不带 `-i`。
- `packages/ui`：包名 `@eyeful/ui`；删除模板的演示页面 `src/routes`；使用 `#lib` 子路径导入，因为 shadcn 命令行在 `$app/tsconfig` 下不接受 `$lib`；`prepare` 时用 `svelte-package` 构建 `dist/`；`src/lib/index.ts` 重新导出各组件、`mode-watcher` 和 `svelte-sonner`。`src/lib/components/ui` 只放 `shadcn-svelte add` 生成的组件，不做修改，不参与 Oxlint 检查，因为 Oxlint 读不了它们的 `.svelte` 导出，改由 svelte-check 覆盖。eyeful 自己的组件（diff 视图、文件树和文件列表）放在旁边的 `src/lib/components`。

## 验证

提交前运行 `mise run check`，其余评审流程见[代码规范与评审](STYLEGUIDE.md#review)。桌面窗口要实际启动来检查：它会打开启动时所在的仓库，在 `mise run dev:desktop` 下就是本仓库，不需要服务端。从窗口发起审查前，像 `local/app` 的测试那样把 `PATH` 指向一个假的 agent 命令行。
