import { bindings, defineConfig, defineContainer, defineWorker, exports } from 'cf/config';
import * as entrypoint from './src/index.ts' with { type: 'cf-worker' };

const server = defineContainer({
	name: 'eyeful-server',
	image: { dockerfile: '../../Dockerfile', buildContext: '../..' },
	instanceType: 'standard-1',
	maxInstances: 5
});

const worker = defineWorker({
	name: 'eyeful',
	compatibilityDate: '2026-10-16',
	entrypoint,
	observability: { enabled: true },
	env: {
		RUNS: bindings.r2({ name: 'eyeful-runs' }),
		EYEFUL_AI_GATEWAY_ID: bindings.text('default'),
		EYEFUL_CLOUDFLARE_ACCOUNT_ID: bindings.secret(),
		EYEFUL_AI_GATEWAY_TOKEN: bindings.secret(),
		EYEFUL_WEBHOOK_SECRET: bindings.secret(),
		EYEFUL_GITHUB_APP_ID: bindings.secret(),
		EYEFUL_GITHUB_APP_PRIVATE_KEY: bindings.secret()
	},
	exports: {
		Server: exports.durableObject({ storage: 'sqlite', container: server }),
		Runs: exports.worker()
	}
});

export default defineConfig({ worker, containers: [server] });
