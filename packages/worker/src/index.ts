import { DurableObject, WorkerEntrypoint } from 'cloudflare:workers';
import type { DatabaseQueryParams, QueryResult } from 'cloudflare/resources/d1/database';
import type { ResponseInfo } from 'cloudflare/resources/shared';

const PORT = 8080;
const IDLE_MS = 10 * 60 * 1000;
const WATCH_MS = 60 * 1000;
const D1_HOST = 'd1.eyeful';
const D1_ACCOUNT = 'worker';
const D1_DATABASE = 'DB';
const D1_PATH = `/client/v4/accounts/${D1_ACCOUNT}/d1/database/${D1_DATABASE}/query`;

type D1Props = { token: string };

type D1Envelope = {
	success: boolean;
	errors: ResponseInfo[];
	messages: ResponseInfo[];
	result: QueryResult[];
};

export class D1Query extends WorkerEntrypoint<Env, D1Props> {
	async fetch(request: Request): Promise<Response> {
		if (request.method !== 'POST' || new URL(request.url).pathname !== D1_PATH) {
			return new Response(null, { status: 404 });
		}
		if (request.headers.get('Authorization') !== `Bearer ${this.ctx.props.token}`) {
			return new Response(null, { status: 401 });
		}
		const query = await request.json<Omit<DatabaseQueryParams.D1SingleQuery, 'account_id'>>();
		const prepared = this.env.DB.prepare(query.sql);
		const statement = query.params === undefined ? prepared : prepared.bind(...query.params);
		try {
			const { results, success } = await statement.run();
			return Response.json({
				success: true,
				errors: [],
				messages: [],
				result: [{ results, success }]
			} satisfies D1Envelope);
		} catch (error) {
			if (!(error instanceof Error)) {
				throw error;
			}
			return Response.json(
				{
					success: false,
					errors: [{ code: 7500, message: error.message }],
					messages: [],
					result: []
				} satisfies D1Envelope,
				{ status: 400 }
			);
		}
	}
}

export class Server extends DurableObject<Env> {
	#ready: Promise<void> | undefined;

	constructor(ctx: DurableObjectState, env: Env) {
		super(ctx, env);
		const container = ctx.container;
		if (container !== undefined && container.running) {
			void ctx.blockConcurrencyWhile(() => container.setInactivityTimeout(IDLE_MS));
		}
	}

	async fetch(request: Request): Promise<Response> {
		const container = this.ctx.container;
		if (container === undefined) {
			throw new Error('no container is configured for this durable object');
		}
		if (this.#ready === undefined) {
			this.#ready = (async () => {
				if (!container.running) {
					const token = crypto.randomUUID();
					await container.interceptOutboundHttp(
						D1_HOST,
						this.ctx.exports.D1Query({ props: { token } })
					);
					const env: Record<string, string> = {};
					for (const [name, value] of Object.entries(this.env)) {
						if (typeof value === 'string') {
							env[name] = value;
						}
					}
					container.start({
						enableInternet: true,
						env: {
							...env,
							EYEFUL_D1_API: `http://${D1_HOST}/client/v4`,
							EYEFUL_D1_ACCOUNT_ID: D1_ACCOUNT,
							EYEFUL_D1_DATABASE_ID: D1_DATABASE,
							EYEFUL_D1_API_TOKEN: token
						}
					});
				}
				await container.setInactivityTimeout(IDLE_MS);
				const port = container.getTcpPort(PORT);
				let refused: Error | undefined;
				for (let attempt = 0; attempt < 150; attempt++) {
					let res: Response;
					try {
						res = await port.fetch('http://container/health', {
							signal: AbortSignal.timeout(1000)
						});
					} catch (error) {
						if (!(error instanceof Error)) {
							throw error;
						}
						refused = error;
						await scheduler.wait(200);
						continue;
					}
					if (res.body !== null) {
						await res.body.cancel();
					}
					if (!res.ok) {
						throw new Error(`health check returned ${res.status}`);
					}
					return;
				}
				throw new Error(`the server did not answer on port ${PORT}`, { cause: refused });
			})().finally(() => {
				this.#ready = undefined;
			});
		}
		await this.#ready;
		if ((await this.ctx.storage.getAlarm()) === null) {
			await this.ctx.storage.setAlarm(Date.now() + WATCH_MS);
		}
		const url = new URL(request.url);
		url.protocol = 'http:';
		url.host = 'container';
		const forwarded = new Request(url, request);
		forwarded.headers.delete('host');
		return container.getTcpPort(PORT).fetch(forwarded);
	}

	async alarm(): Promise<void> {
		const active = await this.env.DB.prepare(
			"SELECT 1 FROM reviews WHERE status IN ('queued', 'running') LIMIT 1"
		).first();
		if (active === null) {
			return;
		}
		const res = await this.fetch(new Request('http://container/health'));
		if (res.body !== null) {
			await res.body.cancel();
		}
		await this.ctx.storage.setAlarm(Date.now() + WATCH_MS);
	}
}

export default {
	fetch(request, _env, ctx) {
		return ctx.exports.Server.getByName('server').fetch(request);
	}
} satisfies ExportedHandler<Env>;
