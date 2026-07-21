import { describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import type { ActivityEvent } from '$lib/activity/client';
import RealtimeSync from './RealtimeSync.svelte';

describe('RealtimeSync', () => {
	it('refreshes application data after a task event', async () => {
		let refreshes = 0;
		render(RealtimeSync, {
			projectId: 'project-id',
			poll: changePoll(testEvent({ type: 'task.created' })),
			refresh: async () => {
				refreshes += 1;
			},
			currentPath: () => '/projects/project-id'
		});

		await vi.waitFor(() => expect(refreshes).toBe(2));
	});

	it('ignores agent lifecycle outside the activity page', async () => {
		let refreshes = 0;
		render(RealtimeSync, {
			projectId: 'project-id',
			poll: changePoll(testEvent({ type: 'agent.run.completed' })),
			refresh: async () => {
				refreshes += 1;
			},
			currentPath: () => '/projects/project-id'
		});

		await new Promise((resolve) => window.setTimeout(resolve, 150));
		expect(refreshes).toBe(1);
	});

	it('refreshes the activity page for every event type', async () => {
		let refreshes = 0;
		render(RealtimeSync, {
			projectId: 'project-id',
			poll: changePoll(testEvent({ type: 'agent.run.completed' })),
			refresh: async () => {
				refreshes += 1;
			},
			currentPath: () => '/projects/project-id/activity'
		});

		await vi.waitFor(() => expect(refreshes).toBe(2));
	});
});

function changePoll(event: ActivityEvent) {
	let calls = 0;
	return async (
		_fetcher: typeof fetch,
		_projectId: string,
		_after: number | null,
		signal: AbortSignal
	) => {
		calls += 1;
		if (calls === 1) return { cursor: 0, events: [] };
		if (calls === 2) {
			return { cursor: event.streamOffset, events: [event] };
		}
		return new Promise<{ cursor: number; events: ActivityEvent[] }>((resolve) =>
			signal.addEventListener('abort', () => resolve({ cursor: event.streamOffset, events: [] }), {
				once: true
			})
		);
	};
}

function testEvent(overrides: Partial<ActivityEvent> = {}): ActivityEvent {
	return {
		streamOffset: 1,
		id: 'event-id',
		type: 'task.updated',
		occurredAt: '2026-07-18T12:00:00Z',
		actorType: 'user',
		actorId: 'user-id',
		source: 'web',
		aggregateType: 'task',
		aggregateId: 'task-id',
		correlationId: 'correlation-id',
		agentRunId: null,
		payload: {},
		...overrides
	};
}
