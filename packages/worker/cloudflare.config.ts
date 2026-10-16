import { bindings, defineConfig, defineContainer, defineWorker, exports } from 'cf/config';
import * as entrypoint from './src/index.ts' with { type: 'cf-worker' };

const server = defineContainer({
	name: 'eyeful-server',
	image: { dockerfile: '../../Dockerfile', buildContext: '../..' },
	instanceType: 'standard-1',
	maxInstances: 1
});

const worker = defineWorker({
	name: 'eyeful',
	compatibilityDate: '2026-10-16',
	entrypoint,
	observability: { enabled: true },
	env: {
		DB: bindings.d1({ name: 'eyeful' }),
		EYEFUL_WEBHOOK_SECRET: bindings.secret(),
		EYEFUL_GITHUB_APP_ID: bindings.secret(),
		EYEFUL_GITHUB_APP_PRIVATE_KEY: bindings.secret()
	},
	exports: {
		Server: exports.durableObject({ storage: 'sqlite', container: server }),
		D1Query: exports.worker()
	}
});

export default defineConfig({ worker, containers: [server] });
