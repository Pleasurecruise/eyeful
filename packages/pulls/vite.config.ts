import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite-plus';

export default defineConfig({
	resolve: {
		alias: [
			{
				find: /^(@earendil-works\/pi-(ai|agent-core)|@ai-sdk\/gateway)(\/.*)?$/,
				replacement: fileURLToPath(new URL('src/unused.ts', import.meta.url))
			}
		]
	},
	build: {
		lib: { entry: 'src/index.ts', formats: ['es'], fileName: () => 'core.js' },
		outDir: '../../workflow/pulls',
		emptyOutDir: false,
		minify: false,
		target: 'es2023'
	}
});
