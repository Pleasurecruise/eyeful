import { paraglideVitePlugin } from '@inlang/paraglide-js';
import adapter from '@sveltejs/adapter-static';
import tailwindcss from '@tailwindcss/vite';
import { sveltekit } from '@sveltejs/kit/vite';
import Icons from 'unplugin-icons/vite';
import { defineConfig, lazyPlugins } from 'vite-plus';

export default defineConfig(({ command }) => {
	const plugins = lazyPlugins(() => [
		tailwindcss(),
		paraglideVitePlugin({
			project: './project.inlang',
			outdir: './src/lib/paraglide',
			strategy: ['localStorage', 'preferredLanguage', 'baseLocale'],
			emitTsDeclarations: true
		}),
		Icons({ compiler: 'svelte' }),
		sveltekit({
			compilerOptions: {
				// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
				runes: ({ filename }) =>
					filename.split(/[/\\]/).includes('node_modules') ? undefined : true
			},
			adapter: adapter({ pages: 'dist', assets: 'dist' }),
			router: { type: 'hash' }
		})
	]);
	if (command === 'build') return { plugins };

	const api = process.env.EYEFUL_SERVER_URL;
	if (!api) throw new Error('EYEFUL_SERVER_URL is not set; run `mise run dev:web`');
	return {
		plugins,
		server: {
			proxy: Object.fromEntries(
				[
					'/users',
					'/sessions',
					'/api-keys',
					'/reviews',
					'/repositories',
					'/oauth',
					'/health',
					'/ready',
					'/api'
				].map((path) => [path, api])
			)
		}
	};
});
