import { deleteCurrentSession, getCurrentSession, type User } from '@eyeful/sdk';

class Session {
	user = $state<User | null>(null);

	async load() {
		const result = await getCurrentSession();
		if (!result.response) {
			throw new Error('network error', { cause: result.error });
		}
		if (result.response.status === 401) {
			this.user = null;
			return;
		}
		if (result.error) {
			throw new Error(`${result.response.status} ${result.response.statusText}`);
		}
		this.user = result.data.user;
	}

	async signOut() {
		await deleteCurrentSession();
		this.user = null;
	}
}

export const session = new Session();
