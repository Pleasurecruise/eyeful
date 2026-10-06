<script lang="ts">
	import { Badge } from '@eyeful/ui';
	import { Severity, Verdict, type Finding } from '#bindings/workflow/report';
	import { m } from '#i18n';

	let { finding }: { finding: Finding } = $props();
</script>

<div class="m-2 grid gap-1 rounded-lg border bg-card p-3 font-sans text-sm text-card-foreground">
	<div class="flex flex-wrap items-center gap-1.5">
		<Badge variant={finding.severity === Severity.SeverityImportant ? 'destructive' : 'secondary'}>
			{finding.severity === Severity.SeverityImportant ? m.important() : m.nit()}
		</Badge>
		<Badge variant="outline">
			{finding.verdict === Verdict.VerdictConfirmed ? m.confirmed() : m.plausible()}
		</Badge>
		<span class="font-medium">{finding.short_summary}</span>
	</div>
	<p>{finding.summary}</p>
	<p class="text-muted-foreground">{finding.verification}</p>
</div>
