import { defineConfig } from 'vite-plus';

const ignorePatterns = [
	'**/dist/**',
	'docs/.vitepress/cache/**',
	'docs/.vitepress/dist/**',
	'**/.svelte-kit/**',
	'**/src/lib/paraglide/**',
	'**/.svelte-check/**',
	'packages/sdk/src/**',
	'packages/ui/src/lib/components/ui/**',
	'spec/**',
	'apps/desktop/frontend/bindings/**',
	'apps/desktop/build/**',
	'apps/desktop/bin/**',
	'workflow/prompts/skills/**',
	'workflow/prompts/skills-lock.json',
	'workflow/pulls/core.js'
];

export default defineConfig({
	fmt: {
		useTabs: true,
		singleQuote: true,
		trailingComma: 'none',
		printWidth: 100,
		svelte: true,
		ignorePatterns
	},
	lint: {
		jsPlugins: [{ name: 'vite-plus', specifier: 'vite-plus/oxlint-plugin' }],
		rules: { 'vite-plus/prefer-vite-plus-imports': 'error' },
		options: { typeAware: true, typeCheck: true },
		ignorePatterns
	}
});
