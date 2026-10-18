# Local development

## Setup {#setup}

```sh
mise trust && mise install   # Go, Node.js, pnpm, golangci-lint
mise run setup               # go mod download + pnpm install
cp .env.example .env         # secrets only; mise loads and redacts it
```

`mise run dev:cloud` runs the cloud locally; it needs Docker, keeps the Durable Objects and R2 in a
local store, and links `packages/worker/.env` to the root `.env` for the secrets. The CLI, the
desktop app and the evaluation keep everything in files.

The desktop app needs the platform's webview: it is built into
macOS, Windows needs WebView2, and Linux needs GTK 4 and WebKitGTK 6.0 (`libgtk-4-dev` and `libwebkitgtk-6.0-dev` on Ubuntu 24.04).

## Commands {#commands}

| Command                                    | Does                                                                                          |
| ------------------------------------------ | --------------------------------------------------------------------------------------------- |
| `mise run dev:cloud`                       | `cf dev`: the Worker on `:5173` (Vite) and the server in a local Container                    |
| `mise run install`                         | The local CLI `eyeful` into `~/.local/bin`, which must be on `PATH`; rerun after a Go upgrade |
| `mise run skills`                          | Reinstall the review skills in `workflow/prompts/skills` from `skills-lock.json`              |
| `mise run dev:desktop` / `package:desktop` | `wails3 dev` (frontend HMR through Vite on `:9245`) / `package`; no server                    |
| `mise run dev:docs` / `build:docs`         | Documentation site (VitePress over `docs/`)                                                   |
| `mise run test`                            | Go tests with `-race`, then the `eval/` scripts' tests                                        |
| `mise run format`                          | gofmt and `vp fmt`                                                                            |
| `mise run lint`                            | gofmt check, golangci-lint, `vp check`, `typecheck` in every package                          |
| `mise run generate`                        | The pulls.review core bundle, the desktop bindings and the Worker types                       |
| `mise run check`                           | lint + test + generate, failing if generated files drift                                      |

The GitHub App's permissions and events are in [Deploy](cloud/DEPLOYMENT.md#deploy).

## Configuration

Values live only at the repository root: credentials in `.env`, shared local settings in
`mise.toml`. Every variable and flag is listed in [Configuration](CONFIGURATION.md#server).

## Code generation

`packages/pulls` (`generate:pulls`) bundles `@pulls.review/core` into `workflow/pulls/core.js`.
`wails3 generate bindings` (`generate:bindings`) writes the desktop's services into
`apps/desktop/frontend/bindings`, the Wails default, as classes, so a Go `nil` slice arrives as
`[]`; git ignores that directory, and `lint`,
`wails3 build` and `wails3 dev` regenerate it. `cf workers types` (`generate:worker`) writes the
Worker's `Env` and runtime types into `packages/worker/.cloudflare/types`, also ignored by git. None
of these outputs is edited by hand.
The desktop icon source is `apps/desktop/build/appicon.png`; `generate:icons` regenerates the macOS
`icons.icns` and `Assets.car` plus the Windows `icon.ico` used by dev and packaged builds.

## Translations

The desktop frontend uses [Paraglide JS](https://inlang.com/m/gerre34r/library-inlang-paraglideJs). It
keeps `project.inlang/` and `messages/{en,zh}.json` and compiles them into `src/lib/paraglide`
(ignored by git), through the Vite plugin in `dev` and `build`, and through `prepare` for
`typecheck`. The locale comes from `localStorage`, then the system language, then English, and the
language menu in the header sets it. `packages/ui` holds no text of its own; pages pass translated
strings in. A message added to `en.json` is added to `zh.json` in the same change.

## Toolchain notes {#toolchain-notes}

- Vite+ 1.2.0 was set up with `vp migrate`. The pnpm catalog aliases `vite` to
  `@voidzero-dev/vite-plus-core`.
- Type checks run on TypeScript 7.0.2 (`@typescript/native`, `svelte-check --tsgo`). `typescript`
  stays at 6.0.3, because `svelte-kit sync` needs its JS API, and it is excluded
  from `pnpm update`.
- VitePress is at 2.0.0-alpha.20. Version 1.6.4 fails under Vite+ because it calls the removed
  `transformWithEsbuild`, while 2.x targets the current Vite.
- Wails v3.0.0-beta.30 has no stable release yet.
- Project tools are Go tools, and mise installs only toolchains. The `wails3` CLI is a `tool` line in
  `apps/desktop/go.mod`,
  so it always matches the Wails library. `tools:wails` builds it into `.tools/bin` (ignored by git),
  which `mise.toml` puts on `PATH` inside the repository only, because the Wails Taskfiles call
  `wails3`. The desktop tasks run it first.

## Regenerating the frontend

`apps/desktop/frontend` and `packages/ui` were created by generators:

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

The only changes made on top of the generated code are these:

- In both: exact versions, `@typescript/native`, `typecheck` with `--tsgo`; `vite.config.ts`
  imports from `vite-plus`; `.npmrc`, `.vscode` and the template README removed.
- In `apps/desktop/frontend`: name `@eyeful/desktop`, `@eyeful/ui`, `@wailsio/runtime`;
  `tailwindcss()`; `router: { type: 'hash' }`; adapter output in `dist/` and `touch dist/.gitkeep`
  after `build` (`go:embed` needs the directory to exist); Paraglide as `sv add paraglide` sets it
  up, without its server hooks and URL strategy, since the app is static; `#bindings/*` imports;
  `build:dev` for `wails3 dev`; the review page. The hash router is needed because Wails serves the
  app from `wails://`, whose origin is `null`.
- In `apps/desktop/build`: the name, identifier and version come from `info` in `config.yml`;
  `wails3 task common:update:build-assets` writes them into `Info.plist`, `Info.dev.plist` and the
  Windows and Linux manifests, which are not edited by hand. Bindings are generated without `-i`.
- In `packages/ui`: name `@eyeful/ui`; the template's demo `src/routes` removed; `#lib` subpath
  imports, since the shadcn CLI rejects `$lib` under `$app/tsconfig`; `svelte-package` on `prepare`
  builds `dist/`; `src/lib/index.ts` re-exports the components, `mode-watcher` and `svelte-sonner`.
  `src/lib/components/ui` holds only what `shadcn-svelte add` writes, unmodified, and is excluded
  from Oxlint, which cannot read their `.svelte` exports; svelte-check covers it instead. eyeful's
  own components, the diff view, the file tree and the file list, sit beside it in
  `src/lib/components`.

## Verification

Run `mise run check` before committing. The rest of the review process is in
[Code style](STYLEGUIDE.md#review). To check the desktop window, launch it: it opens on the
repository it was started in, which under `mise run dev:desktop` is this repository, and it needs no
server. Point `PATH` at a fake agent CLI before starting a review from it, as `local/app`'s tests do.
