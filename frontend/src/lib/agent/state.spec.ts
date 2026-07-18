import { describe, expect, it } from 'vitest';
import { applyAgentEvent, stateFromConversation, visibleAgentMessages } from './state';
import type { AgentConversation, AgentEvent } from './client';

describe('agent chat state', () => {
	it('starts after the canonical conversation cursor and ignores replayed events', () => {
		const initial = stateFromConversation(testConversation({ lastStreamOffset: 12 }));

		const replayed = applyAgentEvent(
			initial,
			testEvent({ streamOffset: 12, payload: { delta: 'duplicate' } })
		);

		expect(replayed).toBe(initial);
		expect(visibleAgentMessages(replayed)).toEqual(initial.messages);
	});

	it('combines streamed deltas into one assistant message', () => {
		let state = stateFromConversation(testConversation());
		state = applyAgentEvent(state, testEvent({ streamOffset: 1, payload: { delta: 'Inbox ' } }));
		state = applyAgentEvent(
			state,
			testEvent({ streamOffset: 2, sequence: 2, payload: { delta: 'is clear.' } })
		);

		expect(visibleAgentMessages(state).at(-1)).toMatchObject({
			role: 'assistant',
			content: 'Inbox is clear.',
			runId: 'run-id'
		});
	});

	it('tracks tool lifecycle and terminal run failure', () => {
		let state = stateFromConversation(testConversation({ runs: [testRun()] }));
		state = applyAgentEvent(
			state,
			testEvent({
				streamOffset: 1,
				type: 'agent.tool.started',
				payload: { toolCallId: 'call-id', toolName: 'task_search' }
			})
		);
		state = applyAgentEvent(
			state,
			testEvent({
				streamOffset: 2,
				sequence: 2,
				type: 'agent.tool.completed',
				payload: { toolCallId: 'call-id', toolName: 'task_search', isError: false }
			})
		);
		state = applyAgentEvent(
			state,
			testEvent({
				streamOffset: 3,
				sequence: 3,
				type: 'agent.run.failed',
				payload: { error: { code: 'provider_error', message: 'Provider unavailable' } }
			})
		);

		expect(state.tools).toEqual([
			{ id: 'call-id', runId: 'run-id', name: 'task_search', status: 'completed' }
		]);
		expect(state.runs[0]).toMatchObject({ status: 'failed', error: 'Provider unavailable' });
	});
});

function testConversation(overrides: Partial<AgentConversation> = {}): AgentConversation {
	return {
		session: {
			id: 'session-id',
			createdAt: '2026-07-18T10:00:00Z',
			updatedAt: '2026-07-18T10:00:00Z'
		},
		messages: [],
		runs: [],
		lastStreamOffset: 0,
		...overrides
	};
}

function testRun() {
	return {
		id: 'run-id',
		sessionId: 'session-id',
		status: 'running' as const,
		correlationId: 'correlation-id',
		startedAt: '2026-07-18T10:00:00Z',
		completedAt: null,
		error: null,
		createdAt: '2026-07-18T10:00:00Z',
		updatedAt: '2026-07-18T10:00:00Z'
	};
}

function testEvent(overrides: Partial<AgentEvent> = {}): AgentEvent {
	return {
		streamOffset: 1,
		runId: 'run-id',
		sessionId: 'session-id',
		sequence: 1,
		type: 'agent.message.delta',
		occurredAt: '2026-07-18T10:00:00Z',
		payload: { delta: 'Hello' },
		...overrides
	};
}
