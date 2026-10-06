import type { FileDiffMetadata } from '@pierre/diffs';
import type { GitStatus, GitStatusEntry } from '@pierre/trees';

const statusOf = {
	change: 'modified',
	new: 'added',
	deleted: 'deleted',
	'rename-pure': 'renamed',
	'rename-changed': 'renamed'
} as const satisfies Record<FileDiffMetadata['type'], GitStatus>;

export function gitStatusOf(files: readonly FileDiffMetadata[]): GitStatusEntry[] {
	return files.map((f) => ({ path: f.name, status: statusOf[f.type] }));
}
