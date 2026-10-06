<script lang="ts">
	import '../app.css';
	import favicon from '#lib/assets/favicon.svg';
	import { m } from '#i18n';
	import '#lib/locale.svelte.ts';
	import { session } from '#lib/session.svelte.ts';
	import { ModeWatcher, Toaster, Tooltip } from '@eyeful/ui';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>eyeful</title>
</svelte:head>

<ModeWatcher />
<Toaster richColors />
<Tooltip.Provider>
	{#await session.load() then}
		{@render children()}
	{:catch error}
		<main class="flex min-h-svh items-center justify-center p-4">
			<p role="alert" class="text-destructive">{m.server_unreachable({ error: error.message })}</p>
		</main>
	{/await}
</Tooltip.Provider>
