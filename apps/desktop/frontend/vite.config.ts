import { paraglideVitePlugin } from '@inlang/paraglide-js';
import adapter from '@sveltejs/adapter-static';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import wails from '@wailsio/runtime/plugins/vite';
import { defineConfig, lazyPlugins } from 'vite-plus';

export default defineConfig({
	plugins: lazyPlugins(() => [
		tailwindcss(),
		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide',
			strategy: ['localStorage', 'preferredLanguage', 'baseLocale'],
			emitTsDeclarations: true
		}),
		sveltekit({
			compilerOptions: {
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({ pages: 'dist', assets: 'dist' }),
			router: { type: 'hash' }
		}),
		wails('./bindings')
	])
});
