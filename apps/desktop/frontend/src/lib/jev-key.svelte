<script lang="ts">
	import { Button, Field, Input, Popover, toast } from '@eyeful/ui';
	import { KeyRound } from '@lucide/svelte';
	import { Jev } from '#bindings';
	import { m } from '#i18n';

	let { ready = $bindable(false) }: { ready?: boolean } = $props();

	let open = $state(false);
	let key = $state('');

	async function load() {
		ready = await Jev.Ready();
	}

	async function save() {
		try {
			await Jev.Save(key.trim());
			key = '';
			open = false;
			toast.success(m.jev_saved());
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			toast.error(m.cannot_save_jev(), { description: e.message });
		}
		await load();
	}

	$effect(() => {
		load();
	});
</script>

<Popover.Root bind:open>
	<Popover.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant={ready ? 'ghost' : 'outline'} size="sm" aria-label={m.jev_key()}>
				<KeyRound />
				{ready ? m.jev() : m.no_jev()}
			</Button>
		{/snippet}
	</Popover.Trigger>
	<Popover.Content align="end" class="w-80">
		<form
			onsubmit={(e) => {
				e.preventDefault();
				save();
			}}
		>
			<Field.Set>
				<Field.Legend>{m.jev_key()}</Field.Legend>
				<Field.Group>
					<Field.Field>
						<Field.Label for="jev-key">{m.jev_key()}</Field.Label>
						<Input id="jev-key" type="password" autocomplete="off" bind:value={key} />
						<Field.Description>{m.jev_key_hint()}</Field.Description>
					</Field.Field>
					<Button type="submit" size="sm" disabled={!key.trim()}>{m.save()}</Button>
				</Field.Group>
			</Field.Set>
		</form>
	</Popover.Content>
</Popover.Root>
