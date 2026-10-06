<script lang="ts">
	import { FileTree, type GitStatusEntry } from '@pierre/trees';
	import { mode } from 'mode-watcher';
	import { cn } from '#lib/utils.js';

	let {
		paths,
		gitStatus,
		onselect,
		class: className
	}: {
		paths: readonly string[];
		gitStatus?: readonly GitStatusEntry[];
		onselect?: (paths: readonly string[]) => void;
		class?: string;
	} = $props();

	let host: HTMLDivElement;

	$effect(() => {
		const tree = new FileTree({
			paths,
			gitStatus,
			onSelectionChange: onselect,
			initialExpansion: 'open',
			flattenEmptyDirectories: true,
			search: true,
			unsafeCSS: `:host { color-scheme: ${mode.current ?? 'light dark'}; }`
		});
		tree.render({ containerWrapper: host });
		return () => tree.cleanUp();
	});
</script>

<div
	bind:this={host}
	data-slot="file-tree"
	class={cn(
		'h-full [--trees-accent-override:var(--primary)] [--trees-bg-override:var(--sidebar)] [--trees-border-color-override:var(--sidebar-border)] [--trees-fg-override:var(--sidebar-foreground)]',
		className
	)}
></div>
