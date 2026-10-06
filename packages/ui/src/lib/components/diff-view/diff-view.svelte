<script lang="ts" generics="T = undefined">
	import {
		CodeView,
		type CodeViewOptions,
		type DiffLineAnnotation,
		type FileDiffMetadata
	} from '@pierre/diffs';
	import { getOrCreateWorkerPoolSingleton } from '@pierre/diffs/worker';
	import DiffsWorker from '@pierre/diffs/worker/worker.js?worker';
	import { mode } from 'mode-watcher';
	import { untrack } from 'svelte';
	import { cn } from '#lib/utils.js';

	let {
		files,
		diffStyle = 'split',
		selected,
		annotations = {},
		renderAnnotation,
		class: className
	}: {
		files: readonly FileDiffMetadata[];
		diffStyle?: 'split' | 'unified';
		selected?: string;
		annotations?: Readonly<Partial<Record<string, DiffLineAnnotation<T>[]>>>;
		renderAnnotation?: CodeViewOptions<T, undefined>['renderAnnotation'];
		class?: string;
	} = $props();

	const theme = { light: 'pierre-light', dark: 'pierre-dark' };
	const options: CodeViewOptions<T, undefined> = $derived({
		theme,
		themeType: mode.current ?? 'system',
		diffStyle,
		stickyHeaders: true,
		unsafeCSS: ':host { --diffs-overflow-override: auto; } [data-code] { scrollbar-gutter: auto; }',
		renderAnnotation
	});

	let host: HTMLDivElement;
	let view = $state.raw<CodeView<T>>();

	$effect(() => {
		const pool = getOrCreateWorkerPoolSingleton({
			poolOptions: { workerFactory: () => new DiffsWorker(), poolSize: 4 },
			highlighterOptions: { theme }
		});
		const created = new CodeView<T>(
			untrack(() => options),
			pool
		);
		created.setup(host);
		view = created;
		return () => created.cleanUp();
	});

	$effect(() => {
		view?.setOptions(options);
	});

	$effect(() => {
		view?.setItems(
			files.map((fileDiff) => ({
				id: fileDiff.name,
				type: 'diff',
				fileDiff,
				annotations: annotations[fileDiff.name]
			}))
		);
	});

	$effect(() => {
		if (selected) view?.scrollTo({ type: 'item', id: selected, align: 'start' });
	});
</script>

<div
	bind:this={host}
	data-slot="diff-view"
	class={cn(
		'h-full overflow-auto [scrollbar-color:var(--border)_transparent] [scrollbar-width:thin]',
		className
	)}
></div>
