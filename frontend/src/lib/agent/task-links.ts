import { canonicalTaskPath, parseTaskPath } from '$lib/tasks/navigation';

export interface AgentMessageSegment {
	text: string;
	href?: string;
}

const taskLinkPattern = /\[([^\]\r\n]+)\]\((\/projects\/[^)\s]+\/tasks\/[^)\s]+)\)/g;

export function parseAgentTaskLinks(content: string): AgentMessageSegment[] {
	const segments: AgentMessageSegment[] = [];
	let cursor = 0;

	for (const match of content.matchAll(taskLinkPattern)) {
		const href = match[2];
		const route = parseTaskPath(href);
		if (!route || canonicalTaskPath(route.projectId, route.taskId) !== href) continue;

		const index = match.index ?? 0;
		if (index > cursor) segments.push({ text: content.slice(cursor, index) });
		segments.push({ text: match[1], href });
		cursor = index + match[0].length;
	}

	if (cursor < content.length) segments.push({ text: content.slice(cursor) });
	return segments.length > 0 ? segments : [{ text: content }];
}
