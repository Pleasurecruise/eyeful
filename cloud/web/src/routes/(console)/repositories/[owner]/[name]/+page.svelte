<script lang="ts">
	import {
		getRepository,
		getRepositoryCommit,
		getRepositoryCommitDiff,
		getRepositoryTree,
		type Problem
	} from '@eyeful/sdk';
	import {
		Alert,
		Button,
		DiffView,
		FileTree,
		Skeleton,
		gitStatusOf,
		parsePatchFiles
	} from '@eyeful/ui';
	import { MediaQuery } from 'svelte/reactivity';
	import { m } from '#i18n';
	import { getLocale } from '#i18n/runtime';
	import type { PageProps } from './$types';

	let { params }: PageProps = $props();

	const wide = new MediaQuery('min-width: 768px');

	let selected = $state<string>();
	let split = $state(true);

	const path = $derived({ owner: params.owner, name: params.name });
	const repository = $derived(getRepository({ path }));
</script>

<svelte:head><title>{params.owner}/{params.name} · eyeful</title></svelte:head>

{#snippet failed(title: string, problem: Problem)}
	<Alert.Root variant="destructive">
		<Alert.Title>{title}</Alert.Title>
		<Alert.Description>{problem.detail}</Alert.Description>
	</Alert.Root>
{/snippet}

{#await repository}
	<Skeleton class="h-6 w-1/3" />
{:then repo}
	{#if repo.error}
		{@render failed(m.repository_unavailable(), repo.error)}
	{:else}
		{#await getRepositoryCommit({ path: { ...path, ref: repo.data.default_branch } })}
			<Skeleton class="h-6 w-1/2" />
		{:then commit}
			{#if commit.error}
				{@render failed(m.repository_unavailable(), commit.error)}
			{:else}
				{@const head = commit.data}
				{@const tree = getRepositoryTree({ path: { ...path, sha: head.commit.tree.sha } })}
				{@const diff = getRepositoryCommitDiff({ path: { ...path, ref: head.sha } }).then((r) =>
					r.error
						? { error: r.error }
						: { files: parsePatchFiles(r.data.diff).flatMap((f) => f.files) }
				)}
				<div class="grid gap-4">
					<header class="flex flex-wrap items-baseline gap-x-3 gap-y-1">
						<h1 class="font-semibold tracking-tight">{repo.data.full_name}</h1>
						<span class="text-sm text-muted-foreground">{repo.data.default_branch}</span>
						<a class="font-mono text-sm hover:underline" href={head.html_url}
							>{head.sha.slice(0, 7)}</a
						>
						<span class="min-w-0 flex-1 truncate text-sm">{head.commit.message.split('\n')[0]}</span
						>
						{#if head.commit.author}
							<span class="text-sm text-muted-foreground">
								{head.commit.author.name}
								{#if head.commit.author.date}
									· {new Date(head.commit.author.date).toLocaleString(getLocale())}
								{/if}
							</span>
						{/if}
						{#if wide.current}
							<Button variant="outline" size="sm" onclick={() => (split = !split)}>
								{split ? m.unified() : m.split()}
							</Button>
						{/if}
					</header>
					<div class="flex h-[70svh] min-h-0 overflow-hidden rounded-md border">
						{#if wide.current}
							<aside class="w-72 shrink-0 border-r">
								{#await tree}
									<Skeleton class="m-3 h-40" />
								{:then t}
									{#if t.error}
										{@render failed(m.tree_unavailable(), t.error)}
									{:else}
										{@const paths = t.data.tree.filter((e) => e.type === 'blob').map((e) => e.path)}
										{#await diff}
											<FileTree {paths} onselect={(p) => (selected = p[0])} />
										{:then d}
											{#if d.error}
												<FileTree {paths} onselect={(p) => (selected = p[0])} />
											{:else}
												<FileTree
													{paths}
													gitStatus={gitStatusOf(d.files)}
													onselect={(p) => (selected = p[0])}
												/>
											{/if}
										{/await}
										{#if t.data.truncated}
											<p class="p-2 text-xs text-muted-foreground">{m.tree_truncated()}</p>
										{/if}
									{/if}
								{/await}
							</aside>
						{/if}
						{#await diff}
							<Skeleton class="m-3 h-64 flex-1" />
						{:then d}
							{#if d.error}
								{@render failed(m.diff_unavailable(), d.error)}
							{:else}
								<!-- TODO(review): show the cloud review's findings on these lines once the pipeline reports them. -->
								<DiffView
									class="min-w-0 flex-1"
									files={d.files}
									{selected}
									diffStyle={split && wide.current ? 'split' : 'unified'}
								/>
							{/if}
						{/await}
					</div>
				</div>
			{/if}
		{/await}
	{/if}
{/await}
