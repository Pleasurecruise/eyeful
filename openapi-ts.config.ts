import { defineConfig } from '@hey-api/openapi-ts';

export default defineConfig({
	input: './spec/swagger.json',
	output: 'packages/sdk/src',
	plugins: ['@hey-api/typescript', '@hey-api/sdk', '@hey-api/client-fetch']
});
