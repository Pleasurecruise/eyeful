import { App } from '@octokit/app';
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
	run: number;
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

type Admission = { review: string } | { busy: string } | { skipped: string };

type RunsProps = { review: string };

type JevRequest = { state: Record<string, unknown>; questions: Record<string, unknown> };

const PORT = 8080;
const RUNS_HOST = 'runs.eyeful';
const MODELS_HOST = 'models.eyeful';
const JEV_HOST = 'jev.eyeful';
const MENTION = '@eyeful-review';
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

export class Models extends WorkerEntrypoint<Env> {
	async fetch(request: Request): Promise<Response> {
		const url = new URL(request.url);
		const headers = new Headers(request.headers);
		headers.delete('Authorization');
		headers.set('cf-aig-authorization', `Bearer ${this.env.EYEFUL_AI_GATEWAY_TOKEN}`);
		return fetch(
			`https://gateway.ai.cloudflare.com/v1/${this.env.EYEFUL_CLOUDFLARE_ACCOUNT_ID}/${this.env.EYEFUL_AI_GATEWAY_ID}/custom-volcengine${url.pathname}${url.search}`,
			{ method: request.method, headers, body: request.body }
		);
	}
}

export class Jev extends WorkerEntrypoint<Env> {
	async fetch(request: Request): Promise<Response> {
		return Response.json(
			await this.env.AI.run('typesafe/jev', await request.json<JevRequest>(), {
				gateway: { id: this.env.EYEFUL_AI_GATEWAY_ID }
			})
		);
	}
}

export class Server extends DurableObject<Env> {
	readonly #container: Container;

	constructor(ctx: DurableObjectState, env: Env) {
		super(ctx, env);
		if (ctx.container === undefined) {
			throw new Error('no container is configured for this durable object');
		}
		this.#container = ctx.container;
	}

	async review(source: Source, automatic: boolean): Promise<Admission> {
		const kept = this.ctx.storage.kv.get<Stored>('review');
		if (kept !== undefined && kept.status !== 'done') {
			return { busy: kept.source.head_sha };
		}
		if (automatic && kept !== undefined) {
			return { skipped: 'this pull request was reviewed once; mention the App to review it again' };
		}
		const run = kept === undefined ? 1 : kept.run + 1;
		this.ctx.storage.kv.put<Stored>('review', {
			source,
			run,
			status: 'queued',
			attempts: 0,
			started_at: 0,
			outcome: '',
			reason: ''
		});
		await this.ctx.storage.setAlarm(Date.now());
		return { review: `${this.ctx.id.toString()}/${run}` };
	}

	async cancel(reason: string): Promise<void> {
		const kept = this.ctx.storage.kv.get<Stored>('review');
		if (kept === undefined || kept.status === 'done') {
			return;
		}
		if (this.#container.running) {
			await this.#container.destroy(reason);
		}
		this.ctx.storage.kv.put<Stored>('review', { ...kept, status: 'done', reason });
	}

	async alarm(): Promise<void> {
		const container = this.#container;
		const kept = this.ctx.storage.kv.get<Stored>('review');
		if (kept === undefined || kept.status === 'done') {
			if (container.running) {
				await container.destroy();
			}
			return;
		}
		if (container.running) {
			let res: Response | undefined;
			try {
				res = await container.getTcpPort(PORT).fetch('http://container/review', {
					signal: AbortSignal.timeout(5000)
				});
			} catch (error) {
				if (!(error instanceof Error)) {
					throw error;
				}
			}
			if (res !== undefined && !res.ok) {
				throw new Error(`the server answered ${res.status} for its review`);
			}
			const status = res === undefined ? undefined : await res.json<Status>();
			if (status !== undefined && status.state !== 'running') {
				this.ctx.storage.kv.put<Stored>('review', {
					...kept,
					status: 'done',
					outcome: status.outcome,
					reason: status.reason
				});
				await container.destroy();
				if (status.outcome === '') {
					await this.#report(kept.source, status.reason);
				}
				return;
			}
			if (status === undefined && Date.now() - kept.started_at > START_MS) {
				await container.destroy('the server did not answer');
			}
			await this.ctx.storage.setAlarm(Date.now() + WATCH_MS);
			return;
		}
		if (kept.attempts >= MAX_ATTEMPTS) {
			const reason = `the review's Container was lost ${kept.attempts} times`;
			this.ctx.storage.kv.put<Stored>('review', { ...kept, status: 'done', reason });
			await this.#report(kept.source, reason);
			return;
		}
		const review = `${this.ctx.id.toString()}/${kept.run}`;
		await container.interceptOutboundHttp(RUNS_HOST, this.ctx.exports.Runs({ props: { review } }));
		await container.interceptOutboundHttp(MODELS_HOST, this.ctx.exports.Models({ props: {} }));
		await container.interceptOutboundHttp(JEV_HOST, this.ctx.exports.Jev({ props: {} }));
		container.start({
			enableInternet: true,
			env: {
				EYEFUL_REVIEW: JSON.stringify(kept.source),
				EYEFUL_RUNS_URL: `http://${RUNS_HOST}`,
				EYEFUL_GITHUB_APP_ID: this.env.EYEFUL_GITHUB_APP_ID,
				EYEFUL_GITHUB_APP_PRIVATE_KEY: this.env.EYEFUL_GITHUB_APP_PRIVATE_KEY
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

	async #report(source: Source, reason: string): Promise<void> {
		const app = new App({
			appId: this.env.EYEFUL_GITHUB_APP_ID,
			privateKey: this.env.EYEFUL_GITHUB_APP_PRIVATE_KEY
		});
		const octokit = await app.getInstallationOctokit(source.installation_id);
		const [owner, repo] = source.repo.split('/');
		await octokit.request('POST /repos/{owner}/{repo}/issues/{issue_number}/comments', {
			owner,
			repo,
			issue_number: source.number,
			body: `**eyeful** did not post a review of \`${source.head_sha.slice(0, 7)}\`: ${reason}.`
		});
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
		const app = new App({
			appId: env.EYEFUL_GITHUB_APP_ID,
			privateKey: env.EYEFUL_GITHUB_APP_PRIVATE_KEY,
			webhooks: { secret: env.EYEFUL_WEBHOOK_SECRET }
		});
		if (!(await app.webhooks.verify(payload, signature))) {
			return problem(401, 'unauthorized', 'the webhook signature is invalid');
		}
		const allowed = env.EYEFUL_ALLOWED_USERS.split(',');
		let response = Response.json({ ignored: name });
		app.webhooks.on(
			['pull_request.opened', 'pull_request.reopened', 'pull_request.ready_for_review'],
			async ({ payload: event }) => {
				const pr = event.pull_request;
				if (pr.draft === true && event.action !== 'ready_for_review') {
					response = Response.json({ ignored: 'draft' });
					return;
				}
				if (!allowed.includes(pr.user.login)) {
					response = Response.json({ ignored: `${pr.user.login} is not allowed` });
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
				const admission = await ctx.exports.Server.getByName(
					`${event.repository.full_name}#${pr.number}`
				).review(
					{
						repo: event.repository.full_name,
						base: pr.base.ref,
						head: pr.head.ref,
						number: pr.number,
						base_sha: pr.base.sha,
						head_sha: pr.head.sha,
						clone_url: event.repository.clone_url,
						installation_id: event.installation.id
					},
					true
				);
				response = Response.json(admission, { status: 202 });
			}
		);
		app.webhooks.on('issue_comment.created', async ({ payload: event, octokit }) => {
			const commenter = event.comment.user;
			if (
				event.issue.pull_request === undefined ||
				!event.comment.body.includes(MENTION) ||
				commenter === null ||
				!allowed.includes(commenter.login)
			) {
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
			const owner = event.repository.owner.login;
			const repo = event.repository.name;
			const { data: pr } = await octokit.request('GET /repos/{owner}/{repo}/pulls/{pull_number}', {
				owner,
				repo,
				pull_number: event.issue.number
			});
			if (pr.state === 'closed') {
				response = Response.json({ ignored: 'closed' });
				return;
			}
			await octokit.request('POST /repos/{owner}/{repo}/issues/comments/{comment_id}/reactions', {
				owner,
				repo,
				comment_id: event.comment.id,
				content: 'eyes'
			});
			const admission = await ctx.exports.Server.getByName(
				`${event.repository.full_name}#${pr.number}`
			).review(
				{
					repo: event.repository.full_name,
					base: pr.base.ref,
					head: pr.head.ref,
					number: pr.number,
					base_sha: pr.base.sha,
					head_sha: pr.head.sha,
					clone_url: event.repository.clone_url,
					installation_id: event.installation.id
				},
				false
			);
			if ('busy' in admission) {
				await octokit.request('POST /repos/{owner}/{repo}/issues/{issue_number}/comments', {
					owner,
					repo,
					issue_number: pr.number,
					body: `**eyeful** is still reviewing \`${admission.busy.slice(0, 7)}\`; mention it again once that review is posted.`
				});
			}
			response = Response.json(admission, { status: 202 });
		});
		app.webhooks.on(
			['pull_request.closed', 'pull_request.converted_to_draft'],
			async ({ payload: event }) => {
				await ctx.exports.Server.getByName(
					`${event.repository.full_name}#${event.pull_request.number}`
				).cancel(`pull request ${event.action}`);
				response = Response.json({ cancelled: event.action });
			}
		);
		await app.webhooks.verifyAndReceive({ id, name, payload, signature });
		return response;
	}
} satisfies ExportedHandler<Env>;
