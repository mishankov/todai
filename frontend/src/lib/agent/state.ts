import type { AgentConversation, AgentEvent, AgentMessage, AgentRun } from './client';

export interface AgentToolActivity {
	id: string;
	runId: string;
	name: string;
	status: 'running' | 'completed' | 'failed';
}

export interface AgentChatState {
	sessionId: string;
	messages: AgentMessage[];
	runs: AgentRun[];
	cursor: number;
	deltas: Record<string, string>;
	tools: AgentToolActivity[];
}

export function stateFromConversation(conversation: AgentConversation): AgentChatState {
	return {
		sessionId: conversation.session.id,
		messages: conversation.messages,
		runs: conversation.runs,
		cursor: conversation.lastStreamOffset,
		deltas: {},
		tools: []
	};
}

export function applyAgentEvent(state: AgentChatState, event: AgentEvent): AgentChatState {
	if (event.streamOffset <= state.cursor) return state;
	const next: AgentChatState = {
		...state,
		cursor: event.streamOffset,
		runs: state.runs.map((run) => ({ ...run })),
		deltas: { ...state.deltas },
		tools: state.tools.map((tool) => ({ ...tool }))
	};
	if (!next.runs.some((run) => run.id === event.runId)) {
		next.runs.push({
			id: event.runId,
			sessionId: event.sessionId,
			status: 'queued',
			correlationId: '',
			startedAt: null,
			completedAt: null,
			error: null,
			createdAt: event.occurredAt,
			updatedAt: event.occurredAt
		});
	}

	switch (event.type) {
		case 'agent.run.started':
			updateRun(next, event.runId, 'running');
			break;
		case 'agent.message.delta': {
			const delta = stringPayload(event, 'delta');
			if (delta) next.deltas[event.runId] = (next.deltas[event.runId] ?? '') + delta;
			break;
		}
		case 'agent.tool.started': {
			const toolCallId = stringPayload(event, 'toolCallId');
			const toolName = stringPayload(event, 'toolName');
			if (toolCallId && toolName) {
				next.tools = [
					...next.tools.filter((tool) => tool.id !== toolCallId),
					{ id: toolCallId, runId: event.runId, name: toolName, status: 'running' }
				];
			}
			break;
		}
		case 'agent.tool.completed': {
			const toolCallId = stringPayload(event, 'toolCallId');
			const isError = event.payload.isError === true;
			next.tools = next.tools.map((tool) =>
				tool.id === toolCallId ? { ...tool, status: isError ? 'failed' : 'completed' } : tool
			);
			break;
		}
		case 'agent.run.completed':
			updateRun(next, event.runId, 'completed');
			break;
		case 'agent.run.failed':
			updateRun(next, event.runId, 'failed', eventError(event));
			break;
		case 'agent.run.aborted':
			updateRun(next, event.runId, 'aborted');
			break;
	}
	return next;
}

export function visibleAgentMessages(state: AgentChatState): AgentMessage[] {
	const messages = state.messages.map((message) => {
		const delta = message.role === 'assistant' && message.runId ? state.deltas[message.runId] : '';
		return delta ? { ...message, content: message.content + delta } : message;
	});
	const representedRuns = new Set(
		messages.filter((message) => message.role === 'assistant').map((message) => message.runId)
	);
	for (const [runId, content] of Object.entries(state.deltas)) {
		if (!content || representedRuns.has(runId)) continue;
		messages.push({
			id: `stream-${runId}`,
			sessionId: state.sessionId,
			runId,
			role: 'assistant',
			content,
			createdAt: new Date().toISOString(),
			updatedAt: new Date().toISOString()
		});
	}
	return messages;
}

export function activeAgentRun(state: AgentChatState): AgentRun | null {
	return (
		[...state.runs].reverse().find((run) => run.status === 'queued' || run.status === 'running') ??
		null
	);
}

function updateRun(
	state: AgentChatState,
	runId: string,
	status: AgentRun['status'],
	error: string | null = null
) {
	const index = state.runs.findIndex((run) => run.id === runId);
	if (index === -1) return;
	state.runs[index] = { ...state.runs[index], status, error };
}

function stringPayload(event: AgentEvent, key: string): string {
	const value = event.payload[key];
	return typeof value === 'string' ? value : '';
}

function eventError(event: AgentEvent): string {
	const value = event.payload.error;
	if (typeof value === 'string') return value;
	if (typeof value === 'object' && value !== null && !Array.isArray(value)) {
		const message = (value as Record<string, unknown>).message;
		if (typeof message === 'string') return message;
	}
	return 'The assistant could not finish this request.';
}
