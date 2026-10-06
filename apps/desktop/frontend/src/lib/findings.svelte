<script lang="ts">
	import { Badge, Empty, ScrollArea, Separator, Tabs } from '@eyeful/ui';
	import { ListChecks } from '@lucide/svelte';
	import { Outcome } from '#bindings/workflow';
	import { Severity, Verdict, type Output } from '#bindings/workflow/report';
	import { m } from '#i18n';

	let {
		output,
		log,
		tab = $bindable('findings'),
		onselect
	}: {
		output: Output | null;
		log: string;
		tab?: string;
		onselect: (file: string) => void;
	} = $props();

	const uncertain = $derived.by(() => {
		if (output === null) return [];
		const u = output.uncertainty;
		return [
			...(u.budget_exhausted ? [m.budget_exhausted()] : []),
			...u.absent.map((a) => m.absent(a)),
			...u.unread.map((r) => m.unread(r)),
			...u.unverified.map((v) => m.unverified(v)),
			...(u.dismissed.length ? [m.dismissed({ count: u.dismissed.length })] : []),
			...u.skipped,
			...u.notes
		];
	});

	const outcomes: Record<Outcome, () => string> = {
		[Outcome.$zero]: m.outcome_none,
		[Outcome.OutcomeComplete]: m.outcome_complete,
		[Outcome.OutcomeNone]: m.outcome_none,
		[Outcome.OutcomeCIFailed]: m.outcome_ci_failed,
		[Outcome.OutcomePartial]: m.outcome_partial
	};
</script>

<Tabs.Root bind:value={tab} class="flex h-full min-h-0 flex-col gap-0">
	<div class="border-b p-2">
		<Tabs.List class="w-full">
			<Tabs.Trigger value="findings">
				{m.findings()}
				{#if output}<Badge variant="secondary">{output.findings.length}</Badge>{/if}
			</Tabs.Trigger>
			<Tabs.Trigger value="log">{m.log()}</Tabs.Trigger>
		</Tabs.List>
	</div>
	<Tabs.Content value="findings" class="min-h-0 flex-1">
		{#if output === null}
			<Empty.Root>
				<Empty.Header>
					<Empty.Media variant="icon"><ListChecks /></Empty.Media>
					<Empty.Title>{m.not_reviewed_yet()}</Empty.Title>
					<Empty.Description>{m.not_reviewed_yet_hint()}</Empty.Description>
				</Empty.Header>
			</Empty.Root>
		{:else}
			<ScrollArea class="h-full">
				<div class="grid gap-3 p-3 text-sm">
					<div class="flex flex-wrap items-center gap-1.5">
						<Badge
							variant={output.outcome === Outcome.OutcomeComplete ? 'secondary' : 'destructive'}
						>
							{outcomes[output.outcome]()}
						</Badge>
						<Badge variant="outline">{output.level}</Badge>
						{#if output.not_reviewed.length}
							<span class="text-muted-foreground">
								{m.files_not_reviewed({ count: output.not_reviewed.length })}
							</span>
						{/if}
					</div>
					{#if output.reason}<p class="text-muted-foreground">{output.reason}</p>{/if}
					{#each output.diagnosis.failures as f, i (i)}
						<p><span class="font-medium">{f.job} · {f.step}</span> {f.cause} {f.fix}</p>
					{/each}
					{#if uncertain.length}
						<div class="grid gap-1">
							<span class="font-medium">{m.uncertainty()}</span>
							<ul class="list-disc ps-5 text-muted-foreground">
								{#each uncertain as text, i (i)}<li>{text}</li>{/each}
							</ul>
						</div>
					{/if}
					<Separator />
					{#each output.findings as f, i (i)}
						<button
							type="button"
							class="grid gap-1.5 rounded-lg border p-3 text-left hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
							onclick={() => onselect(f.file)}
						>
							<div class="flex flex-wrap items-center gap-1.5">
								<Badge
									variant={f.severity === Severity.SeverityImportant ? 'destructive' : 'secondary'}
								>
									{f.severity === Severity.SeverityImportant ? m.important() : m.nit()}
								</Badge>
								<Badge variant="outline">
									{f.verdict === Verdict.VerdictConfirmed ? m.confirmed() : m.plausible()}
								</Badge>
								{#if f.category}<Badge variant="outline">{f.category}</Badge>{/if}
							</div>
							<span class="font-medium">{f.short_summary}</span>
							<span class="font-mono text-xs text-muted-foreground">{f.file}:{f.line}</span>
							<p>{f.summary}</p>
							{#if f.failure_scenario}
								<p><span class="font-medium">{m.failure_scenario()}:</span> {f.failure_scenario}</p>
							{/if}
							<p><span class="font-medium">{m.why()}:</span> {f.why}</p>
							<p class="text-muted-foreground">
								<span class="font-medium">{m.how_verified()}:</span>
								{f.verification}
							</p>
							{#if f.experts.length}
								<span class="text-xs text-muted-foreground">{f.experts.join(', ')}</span>
							{/if}
						</button>
					{:else}
						<Empty.Root>
							<Empty.Header>
								<Empty.Title>{m.no_findings()}</Empty.Title>
								<Empty.Description>{m.no_findings_hint()}</Empty.Description>
							</Empty.Header>
						</Empty.Root>
					{/each}
				</div>
			</ScrollArea>
		{/if}
	</Tabs.Content>
	<Tabs.Content value="log" class="min-h-0 flex-1">
		<ScrollArea class="h-full">
			{#if log}
				<pre class="p-3 font-mono text-xs whitespace-pre-wrap text-muted-foreground">{log}</pre>
			{:else}
				<p class="p-3 text-sm text-muted-foreground">{m.empty_log()}</p>
			{/if}
		</ScrollArea>
	</Tabs.Content>
</Tabs.Root>
