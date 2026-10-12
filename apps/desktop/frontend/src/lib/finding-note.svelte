<script lang="ts">
	import { Badge } from '@eyeful/ui';
	import { Kind, Severity } from '#bindings/workflow';
	import { type Finding } from '#bindings/workflow/report';
	import { m } from '#i18n';

	let { finding }: { finding: Finding } = $props();
</script>

<div class="m-2 grid gap-1 rounded-lg border bg-card p-3 font-sans text-sm text-card-foreground">
	<div class="flex flex-wrap items-center gap-1.5">
		<Badge variant={finding.severity === Severity.SeverityError ? 'destructive' : 'secondary'}>
			{finding.severity === Severity.SeverityError
				? m.severity_error()
				: finding.severity === Severity.SeverityWarning
					? m.severity_warning()
					: m.severity_note()}
		</Badge>
		<Badge variant="outline">
			{finding.kind === Kind.KindFail
				? m.kind_fail()
				: finding.kind === Kind.KindOpen
					? m.kind_open()
					: m.kind_review()}
		</Badge>
		<span class="font-medium">{finding.short_summary}</span>
	</div>
	<p>{finding.summary}</p>
	<p class="text-muted-foreground">{finding.verification}</p>
</div>
