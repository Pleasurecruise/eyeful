# 本地开发

## 环境准备 {#setup}

```sh
mise trust && mise install   # Go, Node.js, pnpm, golangci-lint
mise run setup               # go mod download + pnpm install
cp .env.example .env         # secrets only; mise loads and redacts it
```

`mise run dev:cloud` 在本地运行云端，需要 Docker，Durable Object 和 R2 存在本地，并把 `packages/worker/.env` 链接到根目录的 `.env` 来读取 secret。CLI、桌面端和评测都把数据存在文件里。

桌面端需要系统的 webview：macOS 自带；Windows 需要 WebView2；Linux 需要 GTK 4 和 WebKitGTK 6.0（Ubuntu 24.04 上是 `libgtk-4-dev` 和 `libwebkitgtk-6.0-dev`）。

## 常用命令 {#commands}

| 命令                                       | 作用                                                                                      |
| ------------------------------------------ | ----------------------------------------------------------------------------------------- |
| `mise run dev:cloud`                       | `cf dev`：在 `:5173`（Vite）运行 Worker，服务端跑在本地 Container 里                      |
| `mise run install`                         | 把本地命令行 `eyeful` 安装到 `~/.local/bin`，这个目录要在 `PATH` 上；升级 Go 后要重新安装 |
| `mise run skills`                          | 按 `skills-lock.json` 重新安装 `workflow/prompts/skills` 里的审查 skill                   |
| `mise run dev:desktop` / `package:desktop` | 分别执行 `wails3 dev`（前端经 Vite 在 `:9245` 热更新）、`package`；不需要服务端           |
| `mise run dev:docs` / `build:docs`         | 文档站点（VitePress，内容来自 `docs/`）                                                   |
| `mise run test`                            | 带 `-race` 运行 Go 测试，再运行 `eval/` 脚本的测试                                        |
| `mise run format`                          | gofmt 和 `vp fmt`                                                                         |
| `mise run lint`                            | gofmt 检查、golangci-lint、`vp check`，以及每个包的 `typecheck`                           |
| `mise run generate`                        | 生成 pulls.review core 打包文件、桌面端绑定和 Worker 类型                                 |
| `mise run check`                           | lint + test + generate，生成的文件有变化时失败                                            |

GitHub App 的权限和事件见[部署](cloud/DEPLOYMENT.md#deploy)。

## 配置

配置只放在仓库根目录：凭据放 `.env`，本地共用的设置放 `mise.toml`。全部变量和参数见[配置项参考](CONFIGURATION.md#server)。

## 代码生成

`packages/pulls`（`generate:pulls`）把 `@pulls.review/core` 打包成 `workflow/pulls/core.js`。`wails3 generate bindings`（`generate:bindings`）把桌面端的服务以类的形式生成到 `apps/desktop/frontend/bindings`，这是 Wails 的默认位置，Go 的 `nil` 切片到前端是 `[]`；这个目录不进 git，`lint`、`wails3 build` 和 `wails3 dev` 都会重新生成。`cf workers types`（`generate:worker`）把 Worker 的 `Env` 和运行时类型写到 `packages/worker/.cloudflare/types`，同样不进 git。这些生成结果都不要手动修改。
桌面图标的源文件是 `apps/desktop/build/appicon.png`；`generate:icons` 会一并生成 dev 和打包构建使用的 macOS `icons.icns`、`Assets.car`，以及 Windows `icon.ico`。

## 多语言

桌面端前端使用 [Paraglide JS](https://inlang.com/m/gerre34r/library-inlang-paraglideJs)。它有自己的 `project.inlang/` 和 `messages/{en,zh}.json`，编译到 `src/lib/paraglide`（不进 git）：`dev` 和 `build` 时由 Vite 插件编译，`typecheck` 前由 `prepare` 编译。界面语言依次取 `localStorage`、系统语言，最后是英文，可以在页头的语言菜单里切换。`packages/ui` 本身不含文案，页面把翻译好的文字传进去。往 `en.json` 加文案时，要在同一次改动中加到 `zh.json`。

## 工具链说明 {#toolchain-notes}

- Vite+ 1.2.0 通过 `vp migrate` 接入。pnpm catalog 把 `vite` 指向 `@voidzero-dev/vite-plus-core`。
- 类型检查用 TypeScript 7.0.2（`@typescript/native`、`svelte-check --tsgo`）。`typescript` 保持在 6.0.3，因为 `svelte-kit sync` 要用它的 JS API；它不参与 `pnpm update`。
- VitePress 用的是 2.0.0-alpha.20。1.6.4 在 Vite+ 下会出错，因为它调用了已经移除的 `transformWithEsbuild`，2.x 则适配当前的 Vite。
- Wails v3.0.0-beta.30 还没有正式版。
- 项目用到的工具都是 Go 工具，mise 只安装工具链。`wails3` 命令行写在 `apps/desktop/go.mod` 的 `tool` 中，版本始终和 Wails 库一致。`tools:wails` 把它构建到 `.tools/bin`（不进 git），`mise.toml` 只在仓库目录内把这个目录加进 `PATH`，因为 Wails 的 Taskfile 会调用 `wails3`。桌面端的任务会先执行它。

## 重新生成前端

`apps/desktop/frontend` 和 `packages/ui` 都是生成器创建的：

```sh
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

- 两者都有：使用精确版本、`@typescript/native`、带 `--tsgo` 的 `typecheck`；`vite.config.ts` 从 `vite-plus` 导入；删除 `.npmrc`、`.vscode` 和模板自带的 README。
- `apps/desktop/frontend`：包名 `@eyeful/desktop`，使用 `@eyeful/ui` 和 `@wailsio/runtime`；`tailwindcss()`；`router: { type: 'hash' }`；adapter 输出到 `dist/`，`build` 之后执行 `touch dist/.gitkeep`（`go:embed` 要求目录存在）；按 `sv add paraglide` 的方式接入 Paraglide，但去掉它的服务端 hook 和 URL 策略，因为应用是静态的；`#bindings/*` 导入；供 `wails3 dev` 使用的 `build:dev`；审查页面。这里必须用 hash 路由，因为 Wails 从 `wails://` 提供页面，它的 origin 是 `null`。
- `apps/desktop/build`：名称、标识符和版本来自 `config.yml` 的 `info`，由 `wails3 task common:update:build-assets` 写入 `Info.plist`、`Info.dev.plist` 以及 Windows 和 Linux 的清单，这些文件不手动修改。绑定生成时不带 `-i`。
- `packages/ui`：包名 `@eyeful/ui`；删除模板的演示页面 `src/routes`；使用 `#lib` 子路径导入，因为 shadcn 命令行在 `$app/tsconfig` 下不接受 `$lib`；`prepare` 时用 `svelte-package` 构建 `dist/`；`src/lib/index.ts` 重新导出各组件、`mode-watcher` 和 `svelte-sonner`。`src/lib/components/ui` 只放 `shadcn-svelte add` 生成的组件，不做修改，不参与 Oxlint 检查，因为 Oxlint 读不了它们的 `.svelte` 导出，改由 svelte-check 覆盖。eyeful 自己的组件（diff 视图、文件树和文件列表）放在旁边的 `src/lib/components`。

## 验证

提交前运行 `mise run check`，其余评审流程见[代码规范与评审](STYLEGUIDE.md#review)。桌面窗口要实际启动来检查：它会打开启动时所在的仓库，在 `mise run dev:desktop` 下就是本仓库，不需要服务端。从窗口发起审查前，像 `local/app` 的测试那样把 `PATH` 指向一个假的 agent 命令行。
