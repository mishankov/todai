import { describe, expect, it, vi } from 'vitest';
import type { AgentAPI, AgentEvent, AgentRun } from './client';
import { decomposeTaskWithAgent } from './decompose';

describe('task decomposition', () => {
	it('uses an isolated contextual run and never touches chat state', async () => {
		const api = testAPI();

		await decomposeTaskWithAgent({ id: 'task-id' }, api);

		expect(api.startContextRun).toHaveBeenCalledWith({
			type: 'task',
			taskId: 'task-id',
			action: 'decompose'
		});
		expect(api.streamContextRunEvents).toHaveBeenCalledWith(
			'run-id',
			0,
			expect.any(Function),
			expect.any(AbortSignal)
		);
		expect(api.createSession).not.toHaveBeenCalled();
		expect(api.getSession).not.toHaveBeenCalled();
		expect(api.postMessage).not.toHaveBeenCalled();
		expect(api.streamEvents).not.toHaveBeenCalled();
	});

	it('does not depend on an active run in the global chat', async () => {
		const api = testAPI();
		api.getSession = vi.fn(async () => ({
			session: testSession(),
			messages: [],
			runs: [testRun({ id: 'chat-run', status: 'running' })],
			lastStreamOffset: 12
		}));

		await decomposeTaskWithAgent({ id: 'task-id' }, api);

		expect(api.startContextRun).toHaveBeenCalledOnce();
		expect(api.getSession).not.toHaveBeenCalled();
	});
});

function testAPI(): AgentAPI {
	return {
		createSession: vi.fn(async () => testSession()),
		getSession: vi.fn(),
		startContextRun: vi.fn(async () => testRun()),
		postMessage: vi.fn(),
		abortRun: vi.fn(),
		streamEvents: vi.fn(),
		streamContextRunEvents: vi.fn(async (runId, after, onEvent) => {
			await onEvent(testEvent({ runId, streamOffset: after + 1 }));
			return after + 1;
		})
	};
}

function testRun(overrides: Partial<AgentRun> = {}): AgentRun {
	return {
		id: 'run-id',
		sessionId: 'private-execution-id',
		status: 'queued',
		correlationId: 'correlation-id',
		startedAt: null,
		completedAt: null,
		error: null,
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z',
		...overrides
	};
}

function testSession() {
	return {
		id: 'chat-session-id',
		createdAt: '2026-07-19T10:00:00Z',
		updatedAt: '2026-07-19T10:00:00Z'
	};
}

function testEvent(overrides: Partial<AgentEvent> = {}): AgentEvent {
	return {
		streamOffset: 1,
		runId: 'run-id',
		sessionId: 'private-execution-id',
		sequence: 1,
		type: 'agent.run.completed',
		occurredAt: '2026-07-19T10:00:00Z',
		payload: {},
		...overrides
	};
}
