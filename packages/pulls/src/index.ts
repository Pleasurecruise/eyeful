import type { Analysis } from '@pulls.review/core/llm';
import type { DiffsPayload } from '@pulls.review/core/types';
import * as v from 'valibot';
import {
	AnalysisSchema,
	buildAnalysisPrompt,
	buildCliAgentSystemPrompt,
	describeCoverageIssues,
	findCoverageIssues
} from '@pulls.review/core/llm';

export { parsePatch } from '@pulls.review/core/patch-parser';
export { analysisJsonSchema } from '@pulls.review/core/llm';

export function prompt(diff: DiffsPayload, patchPath: string): string {
	return `${buildCliAgentSystemPrompt({ patchPath, repository: true })}\n\n${buildAnalysisPrompt(diff, 'en')}`;
}

export function check(
	diff: DiffsPayload,
	answer: unknown
): { problem: string } | { analysis: Analysis } {
	const parsed = v.safeParse(AnalysisSchema, answer);
	if (!parsed.success) return { problem: v.summarize(parsed.issues) };
	const problem = describeCoverageIssues(
		findCoverageIssues(diff, parsed.output),
		'Answer again with the whole grouping, every path in exactly one group.'
	);
	return problem ? { problem } : { analysis: parsed.output };
}
