import { Webhooks } from '@octokit/webhooks';
import { DurableObject, WorkerEntrypoint } from 'cloudflare:workers';

type Source = {
	repo: string;
	base: string;
	head: string;
	number: number;
	base_sha: string;
	head_sha: string;
	clone_url: string;
	installation_id: number;
};

type Stored = {
	source: Source;
	status: 'queued' | 'running' | 'done';
	attempts: number;
	started_at: number;
	outcome: string;
	reason: string;
};

type Status = {
	state: 'running' | 'done' | 'failed';
	outcome: string;
	reason: string;
};

type Admission = { review: string; created: boolean } | { conflict: string };

type RunsProps = { review: string };

const PORT = 8080;
const RUNS_HOST = 'runs.eyeful';
const BODY_LIMIT = 5 << 20;
const WATCH_MS = 30 * 1000;
const START_MS = 5 * 60 * 1000;
const IDLE_MS = 10 * 60 * 1000;
const MAX_ATTEMPTS = 3;

export class Runs extends WorkerEntrypoint<Env, RunsProps> {
	async fetch(request: Request): Promise<Response> {
		const segments = new URL(request.url).pathname.slice(1).split('/').map(decodeURIComponent);
		if (segments.some((segment) => segment === '' || segment === '.' || segment === '..')) {
			return new Response(null, { status: 404 });
		}
		const key = `${this.ctx.props.review}/${segments.join('/')}`;
		switch (request.method) {
			case 'GET': {
				const object = await this.env.RUNS.get(key);
				if (object === null) {
					return new Response(null, { status: 404 });
				}
				return new Response(object.body);
			}
			case 'PUT':
				await this.env.RUNS.put(key, await request.arrayBuffer());
				return new Response(null, { status: 204 });
			default:
				return new Response(null, { status: 405 });
		}
	}
}

export class Server extends DurableObject<Env> {
	async admit(source: Source): Promise<Admission> {
		const review = this.ctx.id.toString();
		const kept = this.ctx.storage.kv.get<Stored>('review');
		if (kept !== undefined) {
			if (JSON.stringify(kept.source) !== JSON.stringify(source)) {
				return {
					conflict: 'this delivery does not match the review already queued for that commit'
				};
			}
			return { review, created: false };
		}
		this.ctx.storage.kv.put<Stored>('review', {
			source,
			status: 'queued',
			attempts: 0,
			started_at: 0,
			outcome: '',
			reason: ''
		});
		await this.ctx.storage.setAlarm(Date.now());
		return { review, created: true };
	}

	async alarm(): Promise<void> {
		const container = this.ctx.container;
		if (container === undefined) {
			throw new Error('no container is configured for this durable object');
		}
		const kept = this.ctx.storage.kv.get<Stored>('review');
		if (kept === undefined || kept.status === 'done') {
			if (container.running) {
				await container.destroy();
			}
			return;
		}
		if (container.running) {
			const status = await this.#status(container);
			if (status !== undefined && status.state !== 'running') {
				this.ctx.storage.kv.put<Stored>('review', {
					...kept,
					status: 'done',
					outcome: status.outcome,
					reason: status.reason
				});
				await container.destroy();
				return;
			}
			if (status === undefined && Date.now() - kept.started_at > START_MS) {
				await container.destroy('the server did not answer');
			}
			await this.ctx.storage.setAlarm(Date.now() + WATCH_MS);
			return;
		}
		if (kept.attempts >= MAX_ATTEMPTS) {
			this.ctx.storage.kv.put<Stored>('review', {
				...kept,
				status: 'done',
				reason: `container lost ${kept.attempts} times`
			});
			return;
		}
		const review = this.ctx.id.toString();
		await container.interceptOutboundHttp(RUNS_HOST, this.ctx.exports.Runs({ props: { review } }));
		container.start({
			enableInternet: true,
			env: {
				EYEFUL_REVIEW: JSON.stringify(kept.source),
				EYEFUL_RUNS_URL: `http://${RUNS_HOST}`,
				EYEFUL_GITHUB_APP_ID: this.env.EYEFUL_GITHUB_APP_ID,
				EYEFUL_GITHUB_APP_PRIVATE_KEY: this.env.EYEFUL_GITHUB_APP_PRIVATE_KEY,
				CLOUDFLARE_API_KEY: this.env.EYEFUL_AI_GATEWAY_TOKEN,
				CLOUDFLARE_ACCOUNT_ID: this.env.EYEFUL_CLOUDFLARE_ACCOUNT_ID,
				CLOUDFLARE_GATEWAY_ID: this.env.EYEFUL_AI_GATEWAY_ID
			}
		});
		this.ctx.storage.kv.put<Stored>('review', {
			...kept,
			status: 'running',
			attempts: kept.attempts + 1,
			started_at: Date.now()
		});
		await container.setInactivityTimeout(IDLE_MS);
		await this.ctx.storage.setAlarm(Date.now() + WATCH_MS);
	}

	async #status(container: Container): Promise<Status | undefined> {
		let res: Response;
		try {
			res = await container.getTcpPort(PORT).fetch('http://container/review', {
				signal: AbortSignal.timeout(5000)
			});
		} catch (error) {
			if (!(error instanceof Error)) {
				throw error;
			}
			return undefined;
		}
		if (!res.ok) {
			throw new Error(`the server answered ${res.status} for its review`);
		}
		return res.json<Status>();
	}
}

function problem(status: number, code: string, detail: string): Response {
	return Response.json(
		{ type: 'about:blank', title: code, status, code, detail },
		{ status, headers: { 'Content-Type': 'application/problem+json' } }
	);
}

export default {
	async fetch(request, env, ctx) {
		if (new URL(request.url).pathname !== '/webhook') {
			return problem(404, 'not_found', 'Not Found');
		}
		if (request.method !== 'POST') {
			return problem(405, 'method_not_allowed', 'Method Not Allowed');
		}
		const payload = await request.text();
		if (payload.length > BODY_LIMIT) {
			return problem(413, 'bad_request', 'the webhook body is too large');
		}
		const id = request.headers.get('X-GitHub-Delivery');
		const name = request.headers.get('X-GitHub-Event');
		const signature = request.headers.get('X-Hub-Signature-256');
		if (id === null || name === null || signature === null) {
			return problem(400, 'bad_request', 'the delivery, event or signature header is missing');
		}
		const webhooks = new Webhooks({ secret: env.EYEFUL_WEBHOOK_SECRET });
		if (!(await webhooks.verify(payload, signature))) {
			return problem(401, 'unauthorized', 'the webhook signature is invalid');
		}
		let response = Response.json({ ignored: name });
		webhooks.on(
			[
				'pull_request.opened',
				'pull_request.synchronize',
				'pull_request.reopened',
				'pull_request.ready_for_review'
			],
			async ({ payload: event }) => {
				const pr = event.pull_request;
				if (pr.draft === true && event.action !== 'ready_for_review') {
					response = Response.json({ ignored: 'draft' });
					return;
				}
				if (event.installation === undefined) {
					response = problem(
						400,
						'bad_request',
						'the delivery is not from a GitHub App installation'
					);
					return;
				}
				const source: Source = {
					repo: event.repository.full_name,
					base: pr.base.ref,
					head: pr.head.ref,
					number: pr.number,
					base_sha: pr.base.sha,
					head_sha: pr.head.sha,
					clone_url: event.repository.clone_url,
					installation_id: event.installation.id
				};
				const admission = await ctx.exports.Server.getByName(
					`${source.repo}#${source.number}@${source.head_sha}`
				).admit(source);
				response =
					'conflict' in admission
						? problem(409, 'conflict', admission.conflict)
						: Response.json(admission, { status: 202 });
			}
		);
		await webhooks.verifyAndReceive({ id, name, payload, signature });
		return response;
	}
} satisfies ExportedHandler<Env>;
