<script lang="ts">
	import { listOAuthProviders, type OAuthProvider } from '@eyeful/sdk';
	import { Button, Card, Skeleton } from '@eyeful/ui';
	import { m } from '#i18n';
	import type { Component } from 'svelte';
	import type { SvelteHTMLElements } from 'svelte/elements';
	import Gitee from '~icons/simple-icons/gitee';
	import GitHub from '~icons/simple-icons/github';

	const icons = { github: GitHub, gitee: Gitee } satisfies Record<
		OAuthProvider['id'],
		Component<SvelteHTMLElements['svg']>
	>;

	const providers = listOAuthProviders();
</script>

<main class="flex min-h-svh items-center justify-center p-4">
	<Card.Root class="w-full max-w-sm">
		<Card.Header>
			<Card.Title>{m.sign_in_title()}</Card.Title>
			<Card.Description>{m.tagline()}</Card.Description>
		</Card.Header>
		<Card.Content class="grid gap-3">
			{#await providers}
				<Skeleton class="h-9 w-full" />
			{:then result}
				{#if !result.response}
					<p role="alert" class="text-sm text-destructive">{m.sign_in_unreachable()}</p>
				{:else if result.error}
					<p role="alert" class="text-sm text-destructive">
						{m.sign_in_options_failed({
							status: `${result.response.status} ${result.response.statusText}`
						})}
					</p>
				{:else}
					{#each result.data.items as provider (provider.id)}
						{@const Icon = icons[provider.id]}
						<Button variant="outline" href="/oauth/{provider.id}/authorize" data-sveltekit-reload>
							<Icon />
							{m.sign_in_with({ provider: provider.title })}
						</Button>
					{:else}
						<p role="alert" class="text-sm text-destructive">
							{m.sign_in_no_provider()}
						</p>
					{/each}
				{/if}
			{/await}
		</Card.Content>
	</Card.Root>
</main>
