<script lang="ts">
	import type { GitStatus, GitStatusEntry } from '@pierre/trees';
	import Input from '#lib/components/ui/input/input.svelte';
	import { cn } from '#lib/utils.js';

	let {
		entries,
		selected,
		placeholder,
		onselect,
		class: className
	}: {
		entries: readonly GitStatusEntry[];
		selected?: string;
		placeholder: string;
		onselect: (path: string) => void;
		class?: string;
	} = $props();

	const marks: Record<GitStatus, { letter: string; class: string }> = {
		added: { letter: 'A', class: 'text-green-600 dark:text-green-400' },
		untracked: { letter: 'U', class: 'text-green-600 dark:text-green-400' },
		modified: { letter: 'M', class: 'text-amber-600 dark:text-amber-400' },
		deleted: { letter: 'D', class: 'text-red-600 dark:text-red-400' },
		renamed: { letter: 'R', class: 'text-sky-600 dark:text-sky-400' },
		ignored: { letter: 'I', class: 'text-muted-foreground' }
	};

	let query = $state('');
	const shown = $derived(
		entries.filter((e) => e.path.toLowerCase().includes(query.trim().toLowerCase()))
	);
</script>

<div data-slot="file-list" class={cn('flex h-full flex-col', className)}>
	<div class="p-2">
		<Input bind:value={query} {placeholder} aria-label={placeholder} />
	</div>
	<div
		class="min-h-0 flex-1 overflow-y-auto [scrollbar-color:var(--border)_transparent] [scrollbar-width:thin]"
	>
		<ul class="px-2 pb-2">
			{#each shown as e (e.path)}
				{@const slash = e.path.lastIndexOf('/')}
				<li>
					<button
						type="button"
						title={e.path}
						aria-current={e.path === selected}
						class={cn(
							'flex w-full items-center gap-2 rounded-md px-2 py-1 text-left text-sm hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none',
							e.path === selected && 'bg-muted'
						)}
						onclick={() => onselect(e.path)}
					>
						<span class="flex min-w-0 flex-1">
							<span class="min-w-0 truncate text-muted-foreground"
								>{e.path.slice(0, slash + 1)}</span
							>
							<span class="max-w-full shrink-0 truncate">{e.path.slice(slash + 1)}</span>
						</span>
						<span class={cn('shrink-0 font-mono text-xs', marks[e.status].class)}>
							{marks[e.status].letter}
						</span>
					</button>
				</li>
			{/each}
		</ul>
	</div>
</div>
