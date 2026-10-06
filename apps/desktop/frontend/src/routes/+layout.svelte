<script lang="ts">
	import '../app.css';
	import { ModeWatcher, Toaster, Tooltip } from '@eyeful/ui';
	import '#bindings-events';
	import '#lib/locale.svelte.ts';

	let { children } = $props();
</script>

<svelte:head><title>eyeful</title></svelte:head>

<ModeWatcher />
<Toaster />
<Tooltip.Provider>
	{@render children()}
</Tooltip.Provider>
