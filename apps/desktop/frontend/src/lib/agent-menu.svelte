<script lang="ts">
	import { Button, DropdownMenu, toast } from '@eyeful/ui';
	import { Bot, ChevronDown } from '@lucide/svelte';
	import { Agents } from '#bindings';
	import type { Provider } from '#bindings/local/app';
	import { m } from '#i18n';

	let { connected = $bindable() }: { connected?: string } = $props();

	let providers = $state.raw<Provider[]>([]);

	async function load() {
		providers = await Agents.Providers();
		connected = providers.find((p) => p.Connected)?.Name;
	}

	async function connect(name: string) {
		try {
			const agent = await Agents.Connect(name);
			toast.success(m.connected({ name: agent.Name, version: agent.Version }));
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			toast.error(m.cannot_connect({ name }), { description: e.message });
		}
		await load();
	}

	$effect(() => {
		load();
	});
</script>

<DropdownMenu.Root onOpenChange={(open) => open && load()}>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button {...props} variant="outline" size="sm" aria-label={m.agent()}>
				<Bot />
				{#if connected}{connected}{:else}{m.no_agent()}{/if}
				<ChevronDown />
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="w-64">
		<DropdownMenu.Label>{m.agent()}</DropdownMenu.Label>
		<DropdownMenu.RadioGroup value={connected}>
			{#each providers as p (p.Name)}
				<DropdownMenu.RadioItem
					value={p.Name}
					disabled={!p.Installed}
					onclick={() => p.Name !== connected && connect(p.Name)}
				>
					<div class="grid">
						<span>{p.Name}</span>
						{#if !p.Installed}
							<span class="text-xs text-muted-foreground">{m.not_installed({ tool: p.Tool })}</span>
						{/if}
					</div>
				</DropdownMenu.RadioItem>
			{/each}
		</DropdownMenu.RadioGroup>
	</DropdownMenu.Content>
</DropdownMenu.Root>
