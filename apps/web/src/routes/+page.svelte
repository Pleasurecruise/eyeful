<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { session } from '#lib/session.svelte.ts';

	$effect(() => {
		goto(session.user ? resolve('/(console)/repositories') : resolve('/login'), {
			replaceState: true
		});
	});
</script>
