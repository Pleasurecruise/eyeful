<script lang="ts">
	import { resolve } from '$app/paths';
	import { listRepositories } from '@eyeful/sdk';
	import { Alert, Badge, Button, Skeleton } from '@eyeful/ui';
	import { m } from '#i18n';

	const repositories = listRepositories();
</script>

<svelte:head><title>{m.repositories()} · eyeful</title></svelte:head>

{#await repositories}
	<div class="grid gap-2">
		<Skeleton class="h-10 w-full" />
		<Skeleton class="h-10 w-full" />
	</div>
{:then result}
	{#if result.error}
		<Alert.Root variant="destructive">
			<Alert.Title>{m.repositories_unavailable()}</Alert.Title>
			<Alert.Description>{result.error.detail}</Alert.Description>
		</Alert.Root>
	{:else}
		<div class="grid gap-4">
			<div class="flex flex-wrap items-center gap-3">
				<h1 class="mr-auto font-semibold tracking-tight">{m.repositories()}</h1>
				{#each result.data.installations as installation (installation.id)}
					<Button variant="outline" size="sm" href={installation.html_url}>
						{#if installation.account}
							{m.manage_access_for({ account: installation.account.login })}
						{:else}
							{m.manage_access()}
						{/if}
					</Button>
				{/each}
			</div>
			{#if result.data.installations.length === 0}
				<p class="text-sm text-muted-foreground">{m.not_installed()}</p>
			{:else if result.data.items.length === 0}
				<p class="text-sm text-muted-foreground">{m.no_repositories()}</p>
			{:else}
				<ul class="divide-y rounded-md border">
					{#each result.data.items as repo (repo.full_name)}
						<li>
							<a
								class="flex items-center gap-3 px-4 py-3 hover:bg-muted"
								href={resolve('/(console)/repositories/[owner]/[name]', {
									owner: repo.owner.login,
									name: repo.name
								})}
							>
								<span class="font-medium">{repo.full_name}</span>
								{#if repo.private}
									<Badge variant="secondary">{m.private()}</Badge>
								{/if}
								<span class="ml-auto text-sm text-muted-foreground">{repo.default_branch}</span>
							</a>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
{/await}
