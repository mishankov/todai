import { describe, expect, it, vi } from 'vitest';
import { ActivityRequestError, getActivity, type ActivityEvent } from './client';

describe('activity client', () => {
	it('loads the requested number of activity events', async () => {
		const event = testEvent();
		const fetcher = vi.fn(
			async () =>
				new Response(JSON.stringify({ events: [event] }), {
					status: 200,
					headers: { 'Content-Type': 'application/json' }
				})
		) as unknown as typeof fetch;

		await expect(getActivity(fetcher, 25)).resolves.toEqual([event]);
		expect(fetcher).toHaveBeenCalledWith('/api/activity/?limit=25', {
			credentials: 'same-origin',
			headers: { Accept: 'application/json' }
		});
	});

	it('reports a failed activity request', async () => {
		const fetcher = vi.fn(
			async () => new Response(null, { status: 500 })
		) as unknown as typeof fetch;

		await expect(getActivity(fetcher)).rejects.toBeInstanceOf(ActivityRequestError);
	});
});

function testEvent(): ActivityEvent {
	return {
		id: 'event-id',
		type: 'task.created',
		occurredAt: '2026-07-17T12:00:00Z',
		actorType: 'user',
		actorId: 'user-id',
		source: 'web',
		aggregateType: 'task',
		aggregateId: 'task-id',
		correlationId: 'correlation-id',
		agentRunId: null,
		payload: { schemaVersion: 1, task: { title: 'Plan the day' } }
	};
}
