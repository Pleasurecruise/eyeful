<script lang="ts">
	import {
		Alert,
		AlertDialog,
		Button,
		ButtonGroup,
		DiffView,
		DropdownMenu,
		Field,
		FileList,
		gitStatusOf,
		Input,
		Popover,
		Resizable,
		ScrollArea,
		Skeleton,
		Spinner,
		Tabs,
		Textarea,
		toast,
		parsePatchFiles,
		setMode,
		userPrefersMode,
		type DiffLineAnnotation,
		type FileDiffMetadata,
		type GitStatusEntry
	} from '@eyeful/ui';
	import {
		Check,
		ChevronDown,
		GitBranch,
		Sparkles,
		FolderOpen,
		GitCompare,
		Languages,
		Monitor,
		Moon,
		PanelLeftClose,
		PanelLeftOpen,
		PanelRightClose,
		PanelRightOpen,
		Play,
		Square,
		Sun
	} from '@lucide/svelte';
	import { CancelError, Events, System, type CancellablePromise } from '@wailsio/runtime';
	import { mount, unmount } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { Commit, Review, Worktree } from '#bindings';
	import type { Branch } from '#bindings/local/subject';
	import { Mode, type Expert } from '#bindings/workflow';
	import type { Command } from '#bindings/workflow/project';
	import type { Finding, Output } from '#bindings/workflow/report';
	import { m } from '#i18n';
	import AgentMenu from '#lib/agent-menu.svelte';
	import FindingNote from '#lib/finding-note.svelte';
	import Findings from '#lib/findings.svelte';
	import { switchLocale } from '#lib/locale.svelte.ts';
	import { getLocale, locales } from '#i18n/runtime';

	const wide = new MediaQuery('min-width: 768px');
	const mac = System.IsMac();

	const themes: { mode: Parameters<typeof setMode>[0]; label: () => string; icon: typeof Sun }[] = [
		{ mode: 'light', label: m.theme_light, icon: Sun },
		{ mode: 'dark', label: m.theme_dark, icon: Moon },
		{ mode: 'system', label: m.theme_system, icon: Monitor }
	];

	const modes = [
		{ mode: Mode.ModeReadOnly, label: m.mode_read_only, hint: m.mode_read_only_hint },
		{ mode: Mode.ModeExecution, label: m.mode_execution, hint: m.mode_execution_hint },
		{ mode: Mode.ModeNone, label: m.mode_none, hint: m.mode_none_hint }
	];

	let changes = $state.raw<{
		root: string;
		files: FileDiffMetadata[];
		gitStatus: GitStatusEntry[];
		additions: number;
		deletions: number;
	} | null>(null);
	let error = $state<Error | null>(null);
	let selected = $state<string>();
	let split = $state(true);

	let range = $state({ base: '', head: '' });
	let draft = $state({ base: '', head: '' });
	let subjectOpen = $state(false);
	let branches = $state.raw<Branch[]>([]);
	const current = $derived(branches.find((b) => b.current));

	let summary = $state('');
	let description = $state('');
	let generating = $state.raw<CancellablePromise<string> | null>(null);
	let committing = $state(false);
	let action = $state('review');

	let roster = $state.raw<Expert[]>([]);
	let experts = $state<string[]>([]);
	let mode = $state(modes[0]);
	let agent = $state<string>();

	let pending = $state.raw<CancellablePromise<Output> | null>(null);
	let output = $state.raw<Output | null>(null);
	let failure = $state<Error | null>(null);
	let log = $state('');
	let tab = $state('findings');
	let commands = $state.raw<Command[] | null>(null);

	let treePane = $state<ReturnType<typeof Resizable.Pane>>();
	let reviewPane = $state<ReturnType<typeof Resizable.Pane>>();
	let treeOpen = $state(true);
	let reviewOpen = $state(false);

	const notes: ReturnType<typeof mount>[] = [];

	const annotations = $derived(
		output
			? Object.groupBy(
					output.findings.map((f): DiffLineAnnotation<Finding> => ({
						side: 'additions',
						lineNumber: f.line,
						metadata: f
					})),
					(a) => a.metadata.file
				)
			: {}
	);

	async function load() {
		error = null;
		try {
			const [snapshot, list] = await Promise.all([
				Worktree.Changes(range.base, range.head),
				Worktree.Branches()
			]);
			branches = list;
			const files = parsePatchFiles(snapshot.patch).flatMap((p) => p.files);
			const hunks = files.flatMap((f) => f.hunks);
			changes = {
				root: snapshot.root,
				files,
				gitStatus: gitStatusOf(files),
				additions: hunks.reduce((sum, h) => sum + h.additionLines, 0),
				deletions: hunks.reduce((sum, h) => sum + h.deletionLines, 0)
			};
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			changes = null;
			error = e;
		}
	}

	async function open() {
		try {
			if (!(await Worktree.Open())) return;
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			toast.error(m.cannot_open(), { description: e.message });
		}
		output = null;
		await load();
	}

	async function subject(next: { base: string; head: string }) {
		range = next;
		if (next.base || next.head) action = 'review';
		output = null;
		await load();
	}

	async function useWorktree(dir: string) {
		try {
			await Worktree.Use(dir);
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			toast.error(m.cannot_open(), { description: e.message });
		}
		await subject({ base: '', head: '' });
	}

	async function generate() {
		const run = Commit.Message();
		generating = run;
		try {
			const [first, ...rest] = (await run).split('\n');
			summary = first;
			description = rest.join('\n').trim();
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			if (!(e instanceof CancelError)) toast.error(m.cannot_generate(), { description: e.message });
		} finally {
			generating = null;
		}
	}

	async function commit() {
		committing = true;
		try {
			const head = await Commit.Create(description ? `${summary}\n\n${description}` : summary);
			toast.success(m.committed({ commit: head.slice(0, 7) }));
			summary = '';
			description = '';
			await load();
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			toast.error(m.cannot_commit(), { description: e.message });
		} finally {
			committing = false;
		}
	}

	async function start() {
		output = null;
		failure = null;
		log = '';
		tab = 'log';
		reviewPane?.expand();
		const run = Review.Start(range.base, range.head, experts, mode.mode);
		pending = run;
		try {
			output = await run;
			tab = 'findings';
		} catch (e) {
			if (!(e instanceof Error)) throw e;
			if (!(e instanceof CancelError)) failure = e;
		} finally {
			pending = null;
			commands = null;
		}
	}

	function answer(yes: boolean) {
		commands = null;
		Review.Answer(yes);
	}

	function renderAnnotation({ metadata }: { metadata: Finding }) {
		const target = document.createElement('div');
		notes.push(mount(FindingNote, { target, props: { finding: metadata } }));
		return target;
	}

	$effect(() => {
		load();
	});

	$effect(() => {
		Review.Experts().then((list) => (roster = list));
		const off = [
			Events.On('review:log', (e) => (log += e.data)),
			Events.On('review:confirm', (e) => (commands = e.data))
		];
		return () => off.forEach((f) => f());
	});

	$effect(() => {
		void output;
		return () => notes.splice(0).forEach((n) => unmount(n));
	});
</script>

<svelte:head><title>{m.changes()} · eyeful</title></svelte:head>

<div class="flex h-svh flex-col overflow-hidden bg-background text-foreground">
	<header
		class={[
			'flex h-13 shrink-0 items-center gap-2 border-b px-3 [--wails-draggable:drag]',
			mac && 'ps-20'
		]}
	>
		{#if wide.current}
			<Button
				variant="ghost"
				size="icon-sm"
				class="[--wails-draggable:no-drag]"
				aria-label={m.files()}
				aria-pressed={treeOpen}
				onclick={() => (treeOpen ? treePane?.collapse() : treePane?.expand())}
			>
				{#if treeOpen}<PanelLeftClose />{:else}<PanelLeftOpen />{/if}
			</Button>
		{/if}
		<span class="shrink-0 font-semibold tracking-tight">{m.changes()}</span>
		{#if changes}
			<span class="shrink-0 text-sm tabular-nums">
				{m.file_count({ count: changes.files.length })}
				<span class="text-green-600 dark:text-green-400">+{changes.additions}</span>
				<span class="text-red-600 dark:text-red-400">−{changes.deletions}</span>
			</span>
		{/if}
		<div class="ml-auto flex shrink-0 items-center gap-1 [--wails-draggable:no-drag]">
			{#if wide.current}
				<Button variant="ghost" size="sm" onclick={() => (split = !split)}>
					{split ? m.unified() : m.split()}
				</Button>
			{/if}
			<AgentMenu bind:connected={agent} />
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="icon" aria-label={m.language()}>
							<Languages />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					<DropdownMenu.RadioGroup value={getLocale()}>
						{#each locales as locale (locale)}
							<DropdownMenu.RadioItem value={locale} onclick={() => switchLocale(locale)}>
								{new Intl.DisplayNames([locale], { type: 'language' }).of(locale)}
							</DropdownMenu.RadioItem>
						{/each}
					</DropdownMenu.RadioGroup>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button {...props} variant="ghost" size="icon" aria-label={m.theme()}>
							{#each themes as t (t.mode)}
								{#if userPrefersMode.current === t.mode}<t.icon />{/if}
							{/each}
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end">
					<DropdownMenu.RadioGroup value={userPrefersMode.current}>
						{#each themes as t (t.mode)}
							<DropdownMenu.RadioItem value={t.mode} onSelect={() => setMode(t.mode)}>
								<t.icon />
								{t.label()}
							</DropdownMenu.RadioItem>
						{/each}
					</DropdownMenu.RadioGroup>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
			<Button
				variant="ghost"
				size="icon-sm"
				aria-label={m.review_panel()}
				aria-pressed={reviewOpen}
				onclick={() => (reviewOpen ? reviewPane?.collapse() : reviewPane?.expand())}
			>
				{#if reviewOpen}<PanelRightClose />{:else}<PanelRightOpen />{/if}
			</Button>
		</div>
	</header>

	<div class="flex shrink-0 flex-wrap items-center gap-2 border-b px-4 py-2">
		<Button
			variant="outline"
			size="sm"
			class="max-w-sm min-w-0 shrink justify-start"
			disabled={pending !== null}
			onclick={open}
			title={changes?.root ?? m.open_repository()}
		>
			<FolderOpen />
			<span class="truncate text-muted-foreground">{changes?.root ?? m.open_repository()}</span>
		</Button>
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button {...props} variant="outline" size="sm" disabled={pending !== null}>
						<GitBranch />
						{#if current}{current.name}{:else}{m.detached()}{/if}
						{#if range.head}<span class="text-muted-foreground">{range.head}</span>{/if}
						{#if range.base}<span class="text-muted-foreground">← {range.base}</span>{/if}
						<ChevronDown />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="start" class="w-80">
				<DropdownMenu.Label>{m.worktrees()}</DropdownMenu.Label>
				{#each branches.filter((b) => b.worktree) as b (b.name)}
					<DropdownMenu.Item disabled={b.current} onSelect={() => useWorktree(b.worktree)}>
						<Check class={b.current ? '' : 'invisible'} />
						<div class="grid min-w-0">
							<span>{b.name}</span>
							<span class="truncate text-xs text-muted-foreground">{b.worktree}</span>
						</div>
					</DropdownMenu.Item>
				{/each}
				<DropdownMenu.Separator />
				<DropdownMenu.Label>{m.compare_with()}</DropdownMenu.Label>
				<DropdownMenu.RadioGroup value={range.head ? undefined : range.base}>
					<DropdownMenu.RadioItem value="" onSelect={() => subject({ base: '', head: '' })}>
						{m.uncommitted_only()}
					</DropdownMenu.RadioItem>
					{#each branches.filter((b) => !b.current) as b (b.name)}
						<DropdownMenu.RadioItem
							value={b.name}
							onSelect={() => subject({ base: b.name, head: '' })}
						>
							{b.name}
						</DropdownMenu.RadioItem>
					{/each}
				</DropdownMenu.RadioGroup>
			</DropdownMenu.Content>
		</DropdownMenu.Root>

		<Popover.Root bind:open={subjectOpen} onOpenChange={(o) => o && (draft = { ...range })}>
			<Popover.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						variant="ghost"
						size="icon-sm"
						disabled={pending !== null}
						aria-label={m.custom_range()}
						title={m.custom_range()}
					>
						<GitCompare />
					</Button>
				{/snippet}
			</Popover.Trigger>
			<Popover.Content align="start" class="w-80">
				<form
					onsubmit={(e) => {
						e.preventDefault();
						subjectOpen = false;
						subject({ base: draft.base.trim(), head: draft.head.trim() });
					}}
				>
					<Field.Set>
						<Field.Legend>{m.subject()}</Field.Legend>
						<Field.Group>
							<Field.Field>
								<Field.Label for="base">{m.base()}</Field.Label>
								<Input id="base" bind:value={draft.base} placeholder="main" />
								<Field.Description>{m.base_hint()}</Field.Description>
							</Field.Field>
							<Field.Field>
								<Field.Label for="head">{m.head()}</Field.Label>
								<Input id="head" bind:value={draft.head} placeholder="HEAD" />
								<Field.Description>{m.head_hint()}</Field.Description>
							</Field.Field>
							<Button type="submit" size="sm">{m.apply()}</Button>
						</Field.Group>
					</Field.Set>
				</form>
			</Popover.Content>
		</Popover.Root>

		{#if !wide.current || !treeOpen}
			{@render reviewInline()}
		{/if}
	</div>

	{#if failure}
		<div class="border-b p-4">
			<Alert.Root variant="destructive">
				<Alert.Title>{m.review_failed()}</Alert.Title>
				<Alert.Description>{failure.message}</Alert.Description>
			</Alert.Root>
		</div>
	{/if}

	{#if error}
		<main class="p-4">
			<Alert.Root variant="destructive">
				<Alert.Title>{m.cannot_read()}</Alert.Title>
				<Alert.Description>{error.message}</Alert.Description>
			</Alert.Root>
		</main>
	{:else if changes === null}
		<main class="grid gap-3 p-4">
			<Skeleton class="h-6 w-1/3" />
			<Skeleton class="h-64 w-full" />
		</main>
	{:else if changes.files.length === 0}
		<main class="p-4 text-sm text-muted-foreground">{m.no_changes({ root: changes.root })}</main>
	{:else}
		<Resizable.PaneGroup direction="horizontal" class="min-h-0 flex-1">
			{#if wide.current}
				<Resizable.Pane
					bind:this={treePane}
					id="files"
					order={1}
					defaultSize={20}
					minSize={12}
					collapsible
					collapsedSize={0}
					onCollapse={() => (treeOpen = false)}
					onExpand={() => (treeOpen = true)}
				>
					<div class="flex h-full flex-col">
						<FileList
							class="min-h-0 flex-1"
							entries={changes.gitStatus}
							{selected}
							placeholder={m.filter_files()}
							onselect={(path) => (selected = path)}
						/>
						<Tabs.Root bind:value={action} class="gap-0 border-t">
							<Tabs.List variant="line" class="w-full justify-start border-b px-2">
								<Tabs.Trigger value="review" class="flex-none">{m.review()}</Tabs.Trigger>
								<Tabs.Trigger
									value="commit"
									class="flex-none"
									disabled={range.base !== '' || range.head !== ''}
								>
									{m.commit()}
								</Tabs.Trigger>
							</Tabs.List>
							<div class="grid grid-cols-1 p-3 *:[grid-area:1/1]">
								<div
									role="tabpanel"
									class={['flex flex-col gap-2', action !== 'review' && 'invisible']}
									inert={action !== 'review'}
								>
									{@render reviewPanel()}
								</div>
								<div
									role="tabpanel"
									class={['flex flex-col gap-2', action !== 'commit' && 'invisible']}
									inert={action !== 'commit'}
								>
									{@render commitPanel()}
								</div>
							</div>
						</Tabs.Root>
					</div>
				</Resizable.Pane>
				<Resizable.Handle />
			{/if}
			<Resizable.Pane id="diff" order={2} minSize={30}>
				<DiffView
					files={changes.files}
					{selected}
					{annotations}
					{renderAnnotation}
					diffStyle={split && wide.current ? 'split' : 'unified'}
				/>
			</Resizable.Pane>
			<Resizable.Handle />
			<Resizable.Pane
				bind:this={reviewPane}
				id="review"
				order={3}
				defaultSize={0}
				minSize={24}
				collapsible
				collapsedSize={0}
				onCollapse={() => (reviewOpen = false)}
				onExpand={() => (reviewOpen = true)}
			>
				<Findings {output} {log} bind:tab onselect={(file: string) => (selected = file)} />
			</Resizable.Pane>
		</Resizable.PaneGroup>
	{/if}
</div>

{#snippet modeItems()}
	<DropdownMenu.Label>{m.verification()}</DropdownMenu.Label>
	<DropdownMenu.RadioGroup value={mode.mode}>
		{#each modes as x (x.mode)}
			<DropdownMenu.RadioItem value={x.mode} onSelect={() => (mode = x)}>
				<div class="grid">
					<span>{x.label()}</span>
					<span class="text-xs text-muted-foreground">{x.hint()}</span>
				</div>
			</DropdownMenu.RadioItem>
		{/each}
	</DropdownMenu.RadioGroup>
{/snippet}

{#snippet expertItems()}
	<DropdownMenu.CheckboxItem
		checked={experts.length === 0}
		closeOnSelect={false}
		onCheckedChange={() => (experts = [])}
	>
		{m.planner_chooses()}
	</DropdownMenu.CheckboxItem>
	<DropdownMenu.Separator />
	{#each roster as e (e.name)}
		<DropdownMenu.CheckboxItem
			checked={experts.includes(e.name)}
			closeOnSelect={false}
			onCheckedChange={(on) =>
				(experts = on ? [...experts, e.name] : experts.filter((n) => n !== e.name))}
		>
			<div class="grid">
				<span>{e.name}</span>
				<span class="text-xs text-muted-foreground">{e.area}</span>
			</div>
		</DropdownMenu.CheckboxItem>
	{/each}
{/snippet}

{#snippet startButton(className: string)}
	{#if pending}
		<Button variant="outline" size="sm" class={className} onclick={() => pending?.cancel()}>
			<Spinner />
			{m.reviewing()}
			<Square />
			{m.cancel()}
		</Button>
	{:else}
		<Button size="sm" class={className} disabled={!agent || !changes?.files.length} onclick={start}>
			<Play />
			{#if agent}{m.start_review()}{:else}{m.connect_first()}{/if}
		</Button>
	{/if}
{/snippet}

{#snippet reviewInline()}
	<ButtonGroup.Root class="ml-auto">
		{@render startButton('')}
		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						size="icon-sm"
						disabled={pending !== null}
						aria-label={m.review_options()}
					>
						<ChevronDown />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end" class="w-80">
				{@render modeItems()}
				<DropdownMenu.Separator />
				<DropdownMenu.Sub>
					<DropdownMenu.SubTrigger>{m.experts()}</DropdownMenu.SubTrigger>
					<DropdownMenu.SubContent class="w-72">{@render expertItems()}</DropdownMenu.SubContent>
				</DropdownMenu.Sub>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</ButtonGroup.Root>
{/snippet}

{#snippet reviewPanel()}
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="outline"
					size="sm"
					class="w-full justify-between"
					disabled={pending !== null}
				>
					<span class="text-muted-foreground">{m.experts()}</span>
					<span class="flex min-w-0 items-center gap-1">
						<span class="truncate">
							{experts.length ? m.expert_count({ count: experts.length }) : m.planner_chooses()}
						</span>
						<ChevronDown />
					</span>
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start" class="w-72">
			<DropdownMenu.Label>{m.experts()}</DropdownMenu.Label>
			{@render expertItems()}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="outline"
					size="sm"
					class="w-full justify-between"
					disabled={pending !== null}
				>
					<span class="text-muted-foreground">{m.verification()}</span>
					<span class="flex min-w-0 items-center gap-1">
						<span class="truncate">{mode.label()}</span>
						<ChevronDown />
					</span>
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="start" class="w-80">{@render modeItems()}</DropdownMenu.Content>
	</DropdownMenu.Root>
	{@render startButton('mt-auto w-full')}
{/snippet}

{#snippet commitPanel()}
	<div class="contents">
		<ButtonGroup.Root class="w-full">
			<Input
				bind:value={summary}
				placeholder={m.commit_summary()}
				aria-label={m.commit_summary()}
			/>
			<Button
				variant="outline"
				size="icon"
				disabled={!agent || pending !== null || committing}
				aria-label={generating ? m.cancel() : m.generate_message()}
				title={generating ? m.cancel() : m.generate_message()}
				onclick={() => (generating ? generating.cancel() : generate())}
			>
				{#if generating}<Spinner />{:else}<Sparkles />{/if}
			</Button>
		</ButtonGroup.Root>
		<Textarea
			bind:value={description}
			placeholder={m.commit_description()}
			aria-label={m.commit_description()}
			rows={2}
			class="max-h-32 resize-none"
		/>
		<Button
			size="sm"
			class="mt-auto w-full"
			disabled={!summary.trim() || pending !== null || generating !== null || committing}
			onclick={commit}
		>
			{#if committing}<Spinner />{/if}
			{#if current}{m.commit_to({ branch: current.name })}{:else}{m.commit()}{/if}
		</Button>
	</div>
{/snippet}

<AlertDialog.Root open={commands !== null} onOpenChange={(o) => !o && commands && answer(false)}>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{m.confirm_title()}</AlertDialog.Title>
			<AlertDialog.Description>{m.confirm_description()}</AlertDialog.Description>
		</AlertDialog.Header>
		{#if commands}
			<ScrollArea orientation="horizontal" class="rounded-md bg-muted">
				<pre class="p-3 font-mono text-xs">{commands
						.map((c) => `${c.Name.padEnd(9)} ${c.Run}`)
						.join('\n')}</pre>
			</ScrollArea>
		{/if}
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={() => answer(false)}>{m.cancel()}</AlertDialog.Cancel>
			<AlertDialog.Action onclick={() => answer(true)}>{m.run()}</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
